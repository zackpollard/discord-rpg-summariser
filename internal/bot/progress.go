package bot

import (
	"encoding/json"
	"sync"
	"time"
)

// Pipeline stage weights (approximate relative processing time).
const (
	weightTranscribing = 60
	weightMixing       = 5
	weightSummarising  = 15
	weightEntities     = 7
	weightQuests       = 5
	weightCombat       = 5
	weightEmbeddings   = 3
	totalWeight        = weightTranscribing + weightMixing + weightSummarising +
		weightEntities + weightQuests + weightCombat + weightEmbeddings
)

// stage defines a pipeline stage with its cumulative weight offset.
type stage struct {
	Name   string
	Weight float64
}

var stages = []stage{
	{"transcribing", weightTranscribing},
	{"mixing", weightMixing},
	{"summarising", weightSummarising},
	{"extracting entities", weightEntities},
	{"extracting quests", weightQuests},
	{"extracting combat", weightCombat},
	{"generating embeddings", weightEmbeddings},
}

// ProgressEvent is sent to subscribers over SSE.
type ProgressEvent struct {
	Type    string  `json:"type"` // "progress", "transcript", or "complete"
	Stage   string  `json:"stage,omitempty"`
	Detail  string  `json:"detail,omitempty"`
	Percent float64 `json:"percent"`
	ETA     float64 `json:"eta_seconds"` // -1 = unknown

	// Transcript event fields (type == "transcript").
	Speaker   string  `json:"speaker,omitempty"`
	Text      string  `json:"text,omitempty"`
	StartTime float64 `json:"start_time,omitempty"`
	EndTime   float64 `json:"end_time,omitempty"`
}

// PipelineProgress tracks progress for a single pipeline run and broadcasts
// updates to SSE subscribers.
type PipelineProgress struct {
	mu        sync.RWMutex
	sessionID int64
	startedAt time.Time

	// Current stage tracking.
	stageIdx       int
	completedWeight float64 // sum of completed stage weights
	currentWeight  float64 // weight of current stage
	subProgress    float64 // 0.0-1.0 within current stage
	stage          string
	detail         string

	subscribers map[chan ProgressEvent]struct{}
}

// NewPipelineProgress creates a new progress tracker for the given session.
func NewPipelineProgress(sessionID int64) *PipelineProgress {
	return &PipelineProgress{
		sessionID:   sessionID,
		startedAt:   time.Now(),
		subscribers: make(map[chan ProgressEvent]struct{}),
	}
}

// SessionID returns the session this progress tracker is for.
func (p *PipelineProgress) SessionID() int64 {
	return p.sessionID
}

// SkipStage marks a stage as completed without running it, so the progress
// bar accounts for its weight without getting stuck.
func (p *PipelineProgress) SkipStage(name string) {
	p.mu.Lock()
	for _, s := range stages {
		if s.Name == name {
			p.completedWeight += s.Weight
			break
		}
	}
	p.mu.Unlock()
}

// SetStage transitions to a new pipeline stage.
func (p *PipelineProgress) SetStage(name, detail string) {
	p.mu.Lock()

	// Accumulate weight from the previous stage.
	p.completedWeight += p.currentWeight

	// Find the new stage's weight.
	p.currentWeight = 0
	for i, s := range stages {
		if s.Name == name {
			p.stageIdx = i
			p.currentWeight = s.Weight
			break
		}
	}

	p.stage = name
	p.detail = detail
	p.subProgress = 0
	p.mu.Unlock()

	p.broadcastProgress()
}

// SetDetail updates the detail message without changing the stage.
func (p *PipelineProgress) SetDetail(detail string) {
	p.mu.Lock()
	p.detail = detail
	p.mu.Unlock()
	p.broadcastProgress()
}

// SetSubProgress sets progress within the current stage (0.0-1.0).
func (p *PipelineProgress) SetSubProgress(frac float64) {
	p.mu.Lock()
	p.subProgress = frac
	p.mu.Unlock()
	p.broadcastProgress()
}

// Complete marks the pipeline as finished.
func (p *PipelineProgress) Complete() {
	p.broadcast(ProgressEvent{
		Type:    "complete",
		Percent: 100,
		ETA:     0,
	})
}

// BroadcastTranscript sends a transcript segment to subscribers.
func (p *PipelineProgress) BroadcastTranscript(speaker, text string, startTime, endTime float64) {
	p.broadcast(ProgressEvent{
		Type:      "transcript",
		Speaker:   speaker,
		Text:      text,
		StartTime: startTime,
		EndTime:   endTime,
	})
}

func (p *PipelineProgress) broadcastProgress() {
	p.mu.RLock()
	pct := (p.completedWeight + p.currentWeight*p.subProgress) / float64(totalWeight) * 100
	if pct > 100 {
		pct = 100
	}

	var eta float64 = -1
	if pct > 5 {
		elapsed := time.Since(p.startedAt).Seconds()
		eta = elapsed * (100 - pct) / pct
	}

	evt := ProgressEvent{
		Type:    "progress",
		Stage:   p.stage,
		Detail:  p.detail,
		Percent: pct,
		ETA:     eta,
	}
	p.mu.RUnlock()

	p.broadcast(evt)
}

func (p *PipelineProgress) broadcast(evt ProgressEvent) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for ch := range p.subscribers {
		select {
		case ch <- evt:
		default: // drop if buffer full
		}
	}
}

// Subscribe returns a channel of progress events and an unsubscribe function.
func (p *PipelineProgress) Subscribe() (<-chan ProgressEvent, func()) {
	ch := make(chan ProgressEvent, 64)

	// Send the current state immediately so the client doesn't start blank.
	p.mu.RLock()
	pct := (p.completedWeight + p.currentWeight*p.subProgress) / float64(totalWeight) * 100
	var eta float64 = -1
	if pct > 5 {
		elapsed := time.Since(p.startedAt).Seconds()
		eta = elapsed * (100 - pct) / pct
	}
	initial := ProgressEvent{
		Type:    "progress",
		Stage:   p.stage,
		Detail:  p.detail,
		Percent: pct,
		ETA:     eta,
	}
	p.mu.RUnlock()

	ch <- initial

	p.mu.Lock()
	p.subscribers[ch] = struct{}{}
	p.mu.Unlock()

	return ch, func() {
		p.mu.Lock()
		delete(p.subscribers, ch)
		close(ch)
		p.mu.Unlock()
	}
}

// MarshalEvent serializes a ProgressEvent for SSE.
func (e ProgressEvent) MarshalEvent() ([]byte, error) {
	return json.Marshal(e)
}

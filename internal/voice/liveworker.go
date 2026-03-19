package voice

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"discord-rpg-summariser/internal/audio"
	"discord-rpg-summariser/internal/transcribe"
)

// TranscriptEvent is a live transcript segment broadcast to SSE subscribers.
type TranscriptEvent struct {
	UserID      string  `json:"user_id"`
	DisplayName string  `json:"display_name"`
	StartTime   float64 `json:"start_time"`
	EndTime     float64 `json:"end_time"`
	Text        string  `json:"text"`
}

// LiveWorker reads audio chunks from the channel, transcribes them, and
// broadcasts results to SSE subscribers.
type LiveWorker struct {
	transcriber *transcribe.Transcriber
	chunks      <-chan ChunkReady

	mu          sync.RWMutex
	subscribers map[chan TranscriptEvent]struct{}
	lastPrompt  map[string]string // per-user last text for context continuity
}

func NewLiveWorker(t *transcribe.Transcriber, chunks <-chan ChunkReady) *LiveWorker {
	return &LiveWorker{
		transcriber: t,
		chunks:      chunks,
		subscribers: make(map[chan TranscriptEvent]struct{}),
		lastPrompt:  make(map[string]string),
	}
}

// Run processes chunks until the channel is closed. Call in a goroutine.
func (w *LiveWorker) Run(ctx context.Context) {
	for chunk := range w.chunks {
		w.processChunk(ctx, chunk)
	}
}

func (w *LiveWorker) processChunk(ctx context.Context, chunk ChunkReady) {
	resampled := audio.ResampleChunk(chunk.Samples)
	if len(resampled) == 0 {
		return
	}

	prompt := w.lastPrompt[chunk.UserID]
	segments, err := w.transcriber.TranscribeChunk(ctx, resampled, chunk.StartOffset, prompt)
	if err != nil {
		log.Printf("Live transcribe error for %s: %v", chunk.UserID, err)
		return
	}

	for _, seg := range segments {
		evt := TranscriptEvent{
			UserID:      chunk.UserID,
			DisplayName: chunk.DisplayName,
			StartTime:   seg.StartTime,
			EndTime:     seg.EndTime,
			Text:        seg.Text,
		}
		w.broadcast(evt)
		w.lastPrompt[chunk.UserID] = seg.Text
	}
}

func (w *LiveWorker) broadcast(evt TranscriptEvent) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for ch := range w.subscribers {
		select {
		case ch <- evt:
		default:
			// subscriber too slow, drop event
		}
	}
}

// Subscribe returns a channel that receives live transcript events and an
// unsubscribe function. The channel is buffered and events are dropped if
// the subscriber falls behind.
func (w *LiveWorker) Subscribe() (<-chan TranscriptEvent, func()) {
	ch := make(chan TranscriptEvent, 64)
	w.mu.Lock()
	w.subscribers[ch] = struct{}{}
	w.mu.Unlock()

	return ch, func() {
		w.mu.Lock()
		delete(w.subscribers, ch)
		close(ch)
		w.mu.Unlock()
	}
}

// MarshalEvent is a convenience for SSE serialisation.
func (e TranscriptEvent) MarshalEvent() ([]byte, error) {
	return json.Marshal(e)
}

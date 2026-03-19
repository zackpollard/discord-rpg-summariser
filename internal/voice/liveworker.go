package voice

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

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
	Partial     bool    `json:"partial"`  // true = may be revised by next chunk
	ChunkSeq    int     `json:"chunk_seq"`
}

// LiveWorker reads audio chunks, transcribes them, deduplicates overlapping
// regions, and broadcasts results as partial or confirmed segments.
type LiveWorker struct {
	transcriber *transcribe.Transcriber
	chunks      <-chan ChunkReady

	mu          sync.RWMutex
	subscribers map[chan TranscriptEvent]struct{}

	// Per-user state for overlap deduplication
	lastConfirmedEnd map[string]float64 // user -> end time of last confirmed segment
	lastPrompt       map[string]string  // user -> last text for whisper context
}

func NewLiveWorker(t *transcribe.Transcriber, chunks <-chan ChunkReady) *LiveWorker {
	return &LiveWorker{
		transcriber:      t,
		chunks:           chunks,
		subscribers:      make(map[chan TranscriptEvent]struct{}),
		lastConfirmedEnd: make(map[string]float64),
		lastPrompt:       make(map[string]string),
	}
}

// Run processes chunks until the channel is closed.
func (w *LiveWorker) Run(ctx context.Context) {
	for chunk := range w.chunks {
		w.processChunk(ctx, chunk)
	}
}

func (w *LiveWorker) processChunk(ctx context.Context, chunk ChunkReady) {
	log.Printf("Live transcribing chunk for %s (seq=%d, %.1fs at offset %v)",
		chunk.DisplayName, chunk.ChunkSeq, float64(len(chunk.Samples))/48000.0, chunk.StartOffset)

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

	if len(segments) == 0 {
		return
	}

	log.Printf("Live transcribed %d segments for %s", len(segments), chunk.DisplayName)

	// The overlap region is the first ~2s of this chunk. Segments falling
	// entirely within the overlap were already sent as "partial" by the
	// previous chunk. We skip those and confirm the rest.
	overlapEnd := chunk.StartOffset + time.Duration(overlapSamples)*time.Second/48000
	overlapEndSec := overlapEnd.Seconds()
	confirmedEnd := w.lastConfirmedEnd[chunk.UserID]

	for i, seg := range segments {
		// Skip segments that fall entirely within already-confirmed time
		if seg.EndTime <= confirmedEnd {
			continue
		}

		// Segments in the overlap zone that extend beyond it are confirmed
		// (they were partial before, now we have more context)
		isLast := i == len(segments)-1
		isInNewRegion := seg.StartTime >= overlapEndSec
		partial := isLast && !isInNewRegion // last segment might be cut off

		evt := TranscriptEvent{
			UserID:      chunk.UserID,
			DisplayName: chunk.DisplayName,
			StartTime:   seg.StartTime,
			EndTime:     seg.EndTime,
			Text:        seg.Text,
			Partial:     partial,
			ChunkSeq:    chunk.ChunkSeq,
		}
		w.broadcast(evt)

		if !partial {
			w.lastConfirmedEnd[chunk.UserID] = seg.EndTime
		}
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
		}
	}
}

// Subscribe returns a channel of live transcript events and an unsubscribe func.
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

func (e TranscriptEvent) MarshalEvent() ([]byte, error) {
	return json.Marshal(e)
}

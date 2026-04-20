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
	Partial     bool    `json:"partial"` // true = may be revised by next chunk
	ChunkSeq    int     `json:"chunk_seq"`
}

// SharedMicInfo describes a shared microphone configuration for a Discord user.
type SharedMicInfo struct {
	PartnerUserID      string // real Discord ID or synthetic
	OwnerDisplayName   string // character name or Discord display name
	PartnerDisplayName string // character name or display name
}

// SpeakerIdentifier identifies which speaker is talking in a shared-mic audio chunk.
// It returns the user ID and display name for the identified speaker, or falls back
// to the owner identity if identification isn't possible.
type SpeakerIdentifier interface {
	// IdentifySpeaker takes 16kHz mono float32 audio and the shared mic config,
	// and returns (userID, displayName) for the identified speaker.
	IdentifySpeaker(samples []float32, mic SharedMicInfo) (userID, displayName string)
}

// LiveWorker reads audio chunks, transcribes them, deduplicates overlapping
// regions, and broadcasts results as partial or confirmed segments.
type LiveWorker struct {
	transcriber transcribe.Transcriber
	chunks      <-chan ChunkReady

	// Shared mic support
	sharedMics map[string]SharedMicInfo // discord user ID -> shared mic config
	identifier SpeakerIdentifier        // nil if not available

	mu          sync.RWMutex
	subscribers map[chan TranscriptEvent]struct{}

	// Per-user state for overlap deduplication
	lastConfirmedEnd map[string]float64 // user -> end time of last confirmed segment
	lastPrompt       map[string]string  // user -> last text for whisper context

	done chan struct{} // closed when Run returns
}

func NewLiveWorker(t transcribe.Transcriber, chunks <-chan ChunkReady) *LiveWorker {
	return &LiveWorker{
		transcriber:      t,
		chunks:           chunks,
		sharedMics:       make(map[string]SharedMicInfo),
		subscribers:      make(map[chan TranscriptEvent]struct{}),
		lastConfirmedEnd: make(map[string]float64),
		lastPrompt:       make(map[string]string),
		done:             make(chan struct{}),
	}
}

// SetSharedMics configures shared microphone mappings for live transcription.
func (w *LiveWorker) SetSharedMics(mics map[string]SharedMicInfo) {
	w.sharedMics = mics
}

// SetSpeakerIdentifier sets the speaker identifier used for shared mic chunks.
func (w *LiveWorker) SetSpeakerIdentifier(id SpeakerIdentifier) {
	w.identifier = id
}

// Run processes chunks until the channel is closed.
func (w *LiveWorker) Run(ctx context.Context) {
	defer close(w.done)
	for chunk := range w.chunks {
		w.processChunk(ctx, chunk)
	}
}

// Wait blocks until Run has returned (all in-flight chunks are processed).
func (w *LiveWorker) Wait() {
	<-w.done
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

	// Resolve speaker identity for shared mic users.
	userID := chunk.UserID
	displayName := chunk.DisplayName
	if mic, ok := w.sharedMics[chunk.UserID]; ok {
		userID, displayName = w.resolveSharedMicSpeaker(resampled, mic)
	}

	confirmedEnd := w.lastConfirmedEnd[chunk.UserID]

	for _, seg := range segments {
		// Skip segments we've already confirmed in a previous chunk
		if seg.EndTime <= confirmedEnd {
			continue
		}

		evt := TranscriptEvent{
			UserID:      userID,
			DisplayName: displayName,
			StartTime:   seg.StartTime,
			EndTime:     seg.EndTime,
			Text:        seg.Text,
			Partial:     false,
			ChunkSeq:    chunk.ChunkSeq,
		}
		w.broadcast(evt)
		w.lastConfirmedEnd[chunk.UserID] = seg.EndTime
		w.lastPrompt[chunk.UserID] = seg.Text
	}
}

// resolveSharedMicSpeaker uses the speaker identifier to determine who is
// speaking, or falls back to showing both names.
func (w *LiveWorker) resolveSharedMicSpeaker(samples []float32, mic SharedMicInfo) (string, string) {
	if w.identifier != nil {
		return w.identifier.IdentifySpeaker(samples, mic)
	}
	// Fallback: show both names
	combined := mic.OwnerDisplayName + " & " + mic.PartnerDisplayName
	return "", combined
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

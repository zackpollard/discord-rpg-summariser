package transcribe

import (
	"context"
	"time"
)

// Transcriber is the interface for speech-to-text engines.
type Transcriber interface {
	// TranscribeFile transcribes a 48kHz WAV file and returns timestamped segments.
	TranscribeFile(ctx context.Context, wavPath string) ([]Segment, error)

	// TranscribeChunk transcribes pre-resampled 16kHz float32 mono samples.
	// timeOffset is added to all segment timestamps for session-relative times.
	// prompt provides context from previous chunks for continuity (engine-dependent).
	TranscribeChunk(ctx context.Context, samples []float32, timeOffset time.Duration, prompt string) ([]Segment, error)

	// Close releases engine resources.
	Close() error
}

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

	// SetVocabulary provides campaign-specific words (character names, places,
	// etc.) to bias the transcription model toward recognising them correctly.
	// For Whisper this enriches the initial prompt; for Parakeet it configures
	// hot-word boosting via modified beam search.
	SetVocabulary(words []string)

	// SetGameSystem sets the RPG system name (e.g. "Dungeons & Dragons",
	// "Pathfinder 2e") used in the transcription prompt context.
	SetGameSystem(system string)

	// Close releases engine resources.
	Close() error
}

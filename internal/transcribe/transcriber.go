package transcribe

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"discord-rpg-summariser/internal/audio"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

const modelURLBase = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main"

// Segment represents a single transcribed segment of audio.
type Segment struct {
	StartTime float64
	EndTime   float64
	Text      string
}

// WhisperTranscriber performs speech-to-text using whisper.cpp in-process.
type WhisperTranscriber struct {
	model      whisper.Model
	language   string
	threads    int
	vocabulary []string // campaign-specific words for prompt biasing
	gameSystem string   // RPG system name for prompt context
	mu         sync.Mutex
}

// NewWhisperTranscriber loads the whisper model. If the model file doesn't exist,
// it is downloaded from HuggingFace automatically.
func NewWhisperTranscriber(modelName, modelDir, language string, threads int) (*WhisperTranscriber, error) {
	modelPath := filepath.Join(modelDir, fmt.Sprintf("ggml-%s.bin", modelName))

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		if err := downloadModel(modelName, modelPath); err != nil {
			return nil, fmt.Errorf("download model %s: %w", modelName, err)
		}
	}

	model, err := whisper.New(modelPath)
	if err != nil {
		return nil, fmt.Errorf("load whisper model %s: %w", modelPath, err)
	}

	return &WhisperTranscriber{
		model:    model,
		language: language,
		threads:  threads,
	}, nil
}

// Close releases the whisper model resources.
func (t *WhisperTranscriber) Close() error {
	return t.model.Close()
}

// SetVocabulary stores campaign-specific words to include in the initial
// prompt, biasing Whisper toward recognising them correctly.
func (t *WhisperTranscriber) SetVocabulary(words []string) {
	t.vocabulary = words
}

// SetGameSystem sets the RPG system name for prompt context.
func (t *WhisperTranscriber) SetGameSystem(system string) {
	t.gameSystem = system
}

// buildInitialPrompt constructs a Whisper initial prompt that includes
// the game system and campaign-specific vocabulary when available.
func (t *WhisperTranscriber) buildInitialPrompt() string {
	system := t.gameSystem
	if system == "" {
		system = "Dungeons and Dragons"
	}
	if len(t.vocabulary) == 0 {
		return system + " RPG session with fantasy names and places"
	}
	return system + " RPG session. Names and terms: " + strings.Join(t.vocabulary, ", ") + "."
}

// TranscribeFile transcribes a 48kHz WAV file and returns timestamped segments.
// It streams the file in silence-delimited chunks to avoid loading the entire
// file into memory, and passes the last chunk's text as a prompt for continuity.
func (t *WhisperTranscriber) TranscribeFile(ctx context.Context, wavPath string) ([]Segment, error) {
	var allSegments []Segment
	var lastText string

	err := audio.StreamResample(wavPath, func(samples []float32, offsetSeconds float64) error {
		prompt := t.buildInitialPrompt()
		if lastText != "" {
			prompt = lastText
		}
		segs, err := t.TranscribeChunk(ctx, samples,
			time.Duration(offsetSeconds*float64(time.Second)), prompt)
		if err != nil {
			return err
		}
		allSegments = append(allSegments, segs...)
		if len(segs) > 0 {
			lastText = segs[len(segs)-1].Text
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("stream resample: %w", err)
	}
	return allSegments, nil
}

// TranscribeChunk transcribes pre-resampled 16kHz float32 mono samples.
// timeOffset is added to all segment timestamps for session-relative times.
// prompt provides context from previous chunks for continuity.
func (t *WhisperTranscriber) TranscribeChunk(ctx context.Context, samples []float32, timeOffset time.Duration, prompt string) ([]Segment, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	wctx, err := t.model.NewContext()
	if err != nil {
		return nil, fmt.Errorf("create whisper context: %w", err)
	}

	if err := wctx.SetLanguage(t.language); err != nil {
		return nil, fmt.Errorf("set language: %w", err)
	}
	wctx.SetThreads(uint(t.threads))
	if prompt != "" {
		wctx.SetInitialPrompt(prompt)
	} else {
		wctx.SetInitialPrompt(t.buildInitialPrompt())
	}

	if err := wctx.Process(samples, nil, nil, nil); err != nil {
		return nil, fmt.Errorf("whisper process: %w", err)
	}

	offsetSec := timeOffset.Seconds()
	var segments []Segment
	for {
		seg, err := wctx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read segment: %w", err)
		}
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}
		segments = append(segments, Segment{
			StartTime: seg.Start.Seconds() + offsetSec,
			EndTime:   seg.End.Seconds() + offsetSec,
			Text:      text,
		})
	}
	return segments, nil
}

func downloadModel(name, destPath string) error {
	url := fmt.Sprintf("%s/ggml-%s.bin", modelURLBase, name)
	log.Printf("Downloading whisper model %q from %s...", name, url)

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	n, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write model: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	log.Printf("Downloaded whisper model %q (%d MB)", name, n/1024/1024)
	return nil
}

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

// Transcriber performs speech-to-text using whisper.cpp in-process.
type Transcriber struct {
	model    whisper.Model
	language string
	threads  int
	mu       sync.Mutex // whisper is not thread-safe
}

// NewTranscriber loads the whisper model. If the model file doesn't exist,
// it is downloaded from HuggingFace automatically.
func NewTranscriber(modelName, modelDir, language string, threads int) (*Transcriber, error) {
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

	return &Transcriber{
		model:    model,
		language: language,
		threads:  threads,
	}, nil
}

// Close releases the whisper model resources.
func (t *Transcriber) Close() error {
	return t.model.Close()
}

// TranscribeFile transcribes a 48kHz WAV file and returns timestamped segments.
func (t *Transcriber) TranscribeFile(ctx context.Context, wavPath string) ([]Segment, error) {
	// Resample 48kHz → 16kHz float32 for whisper
	samples, err := audio.LoadAndResample(wavPath)
	if err != nil {
		return nil, fmt.Errorf("load and resample audio: %w", err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	wctx, err := t.model.NewContext()
	if err != nil {
		return nil, fmt.Errorf("create whisper context: %w", err)
	}

	if err := wctx.SetLanguage(t.language); err != nil {
		return nil, fmt.Errorf("set language %s: %w", t.language, err)
	}
	wctx.SetThreads(uint(t.threads))
	wctx.SetInitialPrompt("Dungeons and Dragons RPG session with fantasy names and places")

	if err := wctx.Process(samples, nil, nil, nil); err != nil {
		return nil, fmt.Errorf("whisper process: %w", err)
	}

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
			StartTime: seg.Start.Seconds(),
			EndTime:   seg.End.Seconds(),
			Text:      text,
		})
	}

	return segments, nil
}

// TranscribeChunk transcribes pre-resampled 16kHz float32 mono samples.
// timeOffset is added to all segment timestamps for session-relative times.
// prompt provides context from previous chunks for continuity.
func (t *Transcriber) TranscribeChunk(ctx context.Context, samples []float32, timeOffset time.Duration, prompt string) ([]Segment, error) {
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
		wctx.SetInitialPrompt("Dungeons and Dragons RPG session with fantasy names and places")
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

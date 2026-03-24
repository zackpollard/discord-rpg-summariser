package transcribe

import (
	"archive/tar"
	"compress/bzip2"
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

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

const (
	parakeetModelURL  = "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2"
	parakeetModelDir  = "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8"
	parakeetEncoder   = "encoder.int8.onnx"
	parakeetDecoder   = "decoder.int8.onnx"
	parakeetJoiner    = "joiner.int8.onnx"
	parakeetTokens    = "tokens.txt"
	parakeetModelType = "nemo_transducer"
)

// ParakeetTranscriber performs speech-to-text using NVIDIA Parakeet TDT 0.6B v3
// via sherpa-onnx offline recognition.
type ParakeetTranscriber struct {
	recognizer *sherpa.OfflineRecognizer
	threads    int
	modelBase  string // path to extracted model directory
	mu         sync.Mutex
}

// NewParakeetTranscriber creates a new Parakeet TDT transcriber.
// Models are downloaded automatically on first use.
func NewParakeetTranscriber(modelDir string, threads int) (*ParakeetTranscriber, error) {
	modelBase := filepath.Join(modelDir, parakeetModelDir)
	encoderPath := filepath.Join(modelBase, parakeetEncoder)
	decoderPath := filepath.Join(modelBase, parakeetDecoder)
	joinerPath := filepath.Join(modelBase, parakeetJoiner)
	tokensPath := filepath.Join(modelBase, parakeetTokens)

	// Download models if needed.
	if _, err := os.Stat(encoderPath); os.IsNotExist(err) {
		if err := downloadAndExtractParakeet(modelDir); err != nil {
			return nil, fmt.Errorf("download parakeet model: %w", err)
		}
	}

	config := &sherpa.OfflineRecognizerConfig{}
	config.FeatConfig.SampleRate = 16000
	config.FeatConfig.FeatureDim = 80
	config.ModelConfig.Transducer.Encoder = encoderPath
	config.ModelConfig.Transducer.Decoder = decoderPath
	config.ModelConfig.Transducer.Joiner = joinerPath
	config.ModelConfig.Tokens = tokensPath
	config.ModelConfig.NumThreads = threads
	config.ModelConfig.Provider = "cpu"
	config.ModelConfig.ModelType = parakeetModelType
	config.DecodingMethod = "greedy_search"

	recognizer := sherpa.NewOfflineRecognizer(config)
	if recognizer == nil {
		return nil, fmt.Errorf("failed to create parakeet recognizer (check model paths in %s)", modelBase)
	}

	log.Printf("Parakeet TDT 0.6B v3 model loaded from %s", modelBase)
	return &ParakeetTranscriber{
		recognizer: recognizer,
		threads:    threads,
		modelBase:  modelBase,
	}, nil
}

// Close releases the sherpa-onnx recognizer resources.
func (p *ParakeetTranscriber) Close() error {
	if p.recognizer != nil {
		sherpa.DeleteOfflineRecognizer(p.recognizer)
		p.recognizer = nil
	}
	return nil
}

// SetVocabulary configures hot-word boosting for campaign-specific terms.
// This requires a bpe.vocab file in the model directory (see
// scripts/generate_bpe_vocab.py). When bpe.vocab is absent, this is a no-op.
func (p *ParakeetTranscriber) SetVocabulary(words []string) {
	if len(words) == 0 {
		return
	}

	bpeVocabPath := filepath.Join(p.modelBase, "bpe.vocab")
	if _, err := os.Stat(bpeVocabPath); os.IsNotExist(err) {
		log.Printf("parakeet: bpe.vocab not found at %s — hot words disabled (run scripts/generate_bpe_vocab.py)", bpeVocabPath)
		return
	}

	// Write hotwords file.
	hotwordsPath := filepath.Join(p.modelBase, "hotwords.txt")
	var buf strings.Builder
	for _, w := range words {
		fmt.Fprintf(&buf, "%s :2.5\n", w)
	}
	if err := os.WriteFile(hotwordsPath, []byte(buf.String()), 0o644); err != nil {
		log.Printf("parakeet: failed to write hotwords file: %v", err)
		return
	}

	// Recreate recognizer with modified beam search + hot words.
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.recognizer != nil {
		sherpa.DeleteOfflineRecognizer(p.recognizer)
	}

	config := &sherpa.OfflineRecognizerConfig{}
	config.FeatConfig.SampleRate = 16000
	config.FeatConfig.FeatureDim = 80
	config.ModelConfig.Transducer.Encoder = filepath.Join(p.modelBase, parakeetEncoder)
	config.ModelConfig.Transducer.Decoder = filepath.Join(p.modelBase, parakeetDecoder)
	config.ModelConfig.Transducer.Joiner = filepath.Join(p.modelBase, parakeetJoiner)
	config.ModelConfig.Tokens = filepath.Join(p.modelBase, parakeetTokens)
	config.ModelConfig.NumThreads = p.threads
	config.ModelConfig.Provider = "cpu"
	config.ModelConfig.ModelType = parakeetModelType
	config.ModelConfig.ModelingUnit = "bpe"
	config.ModelConfig.BpeVocab = bpeVocabPath
	config.DecodingMethod = "modified_beam_search"
	config.HotwordsFile = hotwordsPath
	config.HotwordsScore = 2.0

	recognizer := sherpa.NewOfflineRecognizer(config)
	if recognizer == nil {
		log.Printf("parakeet: failed to create recognizer with hot words — falling back to greedy search")
		// Recreate without hot words.
		config.DecodingMethod = "greedy_search"
		config.ModelConfig.ModelingUnit = ""
		config.ModelConfig.BpeVocab = ""
		config.HotwordsFile = ""
		recognizer = sherpa.NewOfflineRecognizer(config)
	} else {
		log.Printf("parakeet: hot words enabled with %d terms", len(words))
	}

	p.recognizer = recognizer
}

// TranscribeFile transcribes a 48kHz WAV file and returns timestamped segments.
// It streams the file in silence-delimited chunks to avoid loading the entire
// file into memory.
func (p *ParakeetTranscriber) TranscribeFile(ctx context.Context, wavPath string) ([]Segment, error) {
	var allSegments []Segment

	err := audio.StreamResample(wavPath, func(samples []float32, offsetSeconds float64) error {
		segs, err := p.TranscribeChunk(ctx, samples,
			time.Duration(offsetSeconds*float64(time.Second)), "")
		if err != nil {
			return err
		}
		allSegments = append(allSegments, segs...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("stream resample: %w", err)
	}
	return allSegments, nil
}

// TranscribeChunk transcribes pre-resampled 16kHz float32 mono samples.
// timeOffset is added to all segment timestamps. The prompt parameter is
// ignored (Parakeet does not support context prompting).
func (p *ParakeetTranscriber) TranscribeChunk(ctx context.Context, samples []float32, timeOffset time.Duration, prompt string) ([]Segment, error) {
	return p.transcribe(samples, timeOffset.Seconds())
}

func (p *ParakeetTranscriber) transcribe(samples []float32, timeOffset float64) ([]Segment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	stream := sherpa.NewOfflineStream(p.recognizer)
	defer sherpa.DeleteOfflineStream(stream)

	stream.AcceptWaveform(16000, samples)
	p.recognizer.Decode(stream)

	result := stream.GetResult()
	if result == nil {
		return nil, nil
	}

	return resultToSegments(result, timeOffset), nil
}

// resultToSegments converts a sherpa-onnx result into timestamped Segment
// values, splitting on sentence boundaries for granularity similar to whisper.
func resultToSegments(result *sherpa.OfflineRecognizerResult, timeOffset float64) []Segment {
	text := strings.TrimSpace(result.Text)
	if text == "" {
		return nil
	}

	tokens := result.Tokens
	timestamps := result.Timestamps
	durations := result.Durations

	// No token-level timing: return the full text as one segment.
	if len(tokens) == 0 || len(timestamps) == 0 {
		return []Segment{{StartTime: timeOffset, EndTime: timeOffset, Text: text}}
	}

	// Walk tokens, accumulate text, and split on sentence-ending punctuation.
	// NeMo BPE tokens use ▁ (U+2581) to mark word boundaries.
	var segments []Segment
	var buf []string
	segStartIdx := 0

	for i, tok := range tokens {
		buf = append(buf, tok)

		trimTok := strings.TrimSpace(tok)
		isSentEnd := strings.HasSuffix(trimTok, ".") ||
			strings.HasSuffix(trimTok, "!") ||
			strings.HasSuffix(trimTok, "?")
		isLast := i == len(tokens)-1

		// Also split on long pauses (>2s gap to next token).
		hasLongPause := false
		if !isLast && i+1 < len(timestamps) {
			curEnd := float64(timestamps[i])
			if i < len(durations) {
				curEnd += float64(durations[i])
			}
			if float64(timestamps[i+1])-curEnd > 2.0 {
				hasLongPause = true
			}
		}

		if isSentEnd || isLast || hasLongPause {
			joined := strings.Join(buf, "")
			// Replace BPE word-boundary marker with space.
			joined = strings.ReplaceAll(joined, "\u2581", " ")
			joined = strings.TrimSpace(joined)

			if joined != "" {
				start := float64(timestamps[segStartIdx]) + timeOffset
				end := float64(timestamps[i]) + timeOffset
				if i < len(durations) {
					end += float64(durations[i])
				}
				segments = append(segments, Segment{
					StartTime: start,
					EndTime:   end,
					Text:      joined,
				})
			}
			buf = nil
			segStartIdx = i + 1
		}
	}

	return segments
}

func downloadAndExtractParakeet(destDir string) error {
	log.Printf("Downloading Parakeet TDT 0.6B v3 model from %s...", parakeetModelURL)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	resp, err := http.Get(parakeetModelURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	if err := extractParakeetTarBz2(resp.Body, destDir); err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	log.Printf("Downloaded and extracted Parakeet TDT model to %s", filepath.Join(destDir, parakeetModelDir))
	return nil
}

func extractParakeetTarBz2(r io.Reader, destDir string) error {
	bzReader := bzip2.NewReader(r)
	tarReader := tar.NewReader(bzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		name := filepath.Clean(header.Name)
		if strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(destDir, name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}
			if _, err := io.Copy(f, tarReader); err != nil {
				f.Close()
				return fmt.Errorf("write file %s: %w", target, err)
			}
			f.Close()
		}
	}
	return nil
}

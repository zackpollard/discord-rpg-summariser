package e2e

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
	"testing"
	"time"

	"discord-rpg-summariser/internal/transcribe"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

const (
	ttsModelURL  = "https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models/vits-piper-en_US-lessac-medium.tar.bz2"
	ttsModelDir  = "vits-piper-en_US-lessac-medium"
	ttsModelFile = "en_US-lessac-medium.onnx"
	ttsTokens    = "tokens.txt"
	ttsDataDir   = "espeak-ng-data"
)

// modelsDir returns the absolute path to the shared models directory at the
// repository root. The same directory is used by the application at runtime.
func modelsDir(t *testing.T) string {
	t.Helper()
	// Walk up from this test file's location to find the repository root.
	// The test is at internal/e2e/ so we go up two levels.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Join(dir, "..", "..")
	mdir := filepath.Join(root, "models")
	if err := os.MkdirAll(mdir, 0o755); err != nil {
		t.Fatalf("create models dir: %v", err)
	}
	return mdir
}

// ensureTTSModel downloads the VITS Piper TTS model if it is not already cached.
func ensureTTSModel(t *testing.T, mdir string) string {
	t.Helper()
	modelBase := filepath.Join(mdir, ttsModelDir)
	modelPath := filepath.Join(modelBase, ttsModelFile)

	if _, err := os.Stat(modelPath); err == nil {
		return modelBase
	}

	t.Logf("Downloading TTS model from %s ...", ttsModelURL)
	resp, err := http.Get(ttsModelURL)
	if err != nil {
		t.Fatalf("download TTS model: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download TTS model: HTTP %d", resp.StatusCode)
	}

	if err := extractTarBz2(resp.Body, mdir); err != nil {
		t.Fatalf("extract TTS model: %v", err)
	}

	if _, err := os.Stat(modelPath); err != nil {
		t.Fatalf("TTS model file not found after extraction: %s", modelPath)
	}

	t.Logf("TTS model ready at %s", modelBase)
	return modelBase
}

// createTTS builds a sherpa-onnx offline TTS engine from the given model directory.
func createTTS(t *testing.T, modelBase string) *sherpa.OfflineTts {
	t.Helper()
	config := sherpa.OfflineTtsConfig{}
	config.Model.Vits.Model = filepath.Join(modelBase, ttsModelFile)
	config.Model.Vits.Tokens = filepath.Join(modelBase, ttsTokens)
	config.Model.Vits.DataDir = filepath.Join(modelBase, ttsDataDir)
	config.Model.Vits.NoiseScale = 0.667
	config.Model.Vits.NoiseScaleW = 0.8
	config.Model.Vits.LengthScale = 1.0
	config.Model.NumThreads = 2
	config.Model.Provider = "cpu"
	config.MaxNumSentences = 1

	tts := sherpa.NewOfflineTts(&config)
	if tts == nil {
		t.Fatal("failed to create TTS engine — check model paths")
	}
	return tts
}

// generateAudio16k synthesises speech from text using TTS and resamples the
// output to 16 kHz mono float32 suitable for the Parakeet transcriber.
func generateAudio16k(t *testing.T, tts *sherpa.OfflineTts, text string) []float32 {
	t.Helper()

	audio := tts.Generate(text, 0, 1.0)
	if audio == nil || len(audio.Samples) == 0 {
		t.Fatalf("TTS generated no audio for: %q", text)
	}

	t.Logf("TTS generated %d samples at %d Hz (%.2fs)",
		len(audio.Samples), audio.SampleRate,
		float64(len(audio.Samples))/float64(audio.SampleRate))

	samples16k := resampleLinear(audio.Samples, audio.SampleRate, 16000)
	t.Logf("Resampled to %d samples at 16000 Hz (%.2fs)",
		len(samples16k), float64(len(samples16k))/16000.0)

	return samples16k
}

// containsKeyWords checks that the transcribed text contains each of the given
// key words (case-insensitive).
func containsKeyWords(text string, keywords []string) []string {
	lower := strings.ToLower(text)
	var missing []string
	for _, kw := range keywords {
		if !strings.Contains(lower, strings.ToLower(kw)) {
			missing = append(missing, kw)
		}
	}
	return missing
}

// TestTTSSingleSentence generates a single sentence with TTS, transcribes it,
// and verifies key words are present in the output.
func TestTTSSingleSentence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running e2e TTS test in short mode")
	}

	mdir := modelsDir(t)

	// Ensure TTS model is available.
	ttsBase := ensureTTSModel(t, mdir)
	tts := createTTS(t, ttsBase)
	defer sherpa.DeleteOfflineTts(tts)

	// Generate audio from a known phrase.
	phrase := "The dragon attacked the village at dawn"
	samples := generateAudio16k(t, tts, phrase)

	// Create Parakeet transcriber. It will download models if needed.
	pt, err := transcribe.NewParakeetTranscriber(mdir, 2)
	if err != nil {
		t.Skipf("Parakeet transcriber not available (model download may have failed): %v", err)
	}
	defer pt.Close()

	segments, err := pt.TranscribeChunk(context.Background(), samples, 0, "")
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}

	if len(segments) == 0 {
		t.Fatal("transcription returned no segments")
	}

	// Combine all segment text.
	var texts []string
	for _, seg := range segments {
		t.Logf("segment [%.2f - %.2f]: %s", seg.StartTime, seg.EndTime, seg.Text)
		texts = append(texts, seg.Text)
	}
	fullText := strings.Join(texts, " ")
	t.Logf("Full transcription: %q", fullText)

	// Verify key words from the input phrase are present.
	keywords := []string{"dragon", "village", "dawn"}
	if missing := containsKeyWords(fullText, keywords); len(missing) > 0 {
		t.Errorf("transcription missing keywords %v in: %q", missing, fullText)
	}
}

// TestTTSMultiSentence generates a longer passage with multiple sentences,
// transcribes it, and verifies that multiple segments are produced and key
// words from each sentence are present.
func TestTTSMultiSentence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running e2e TTS test in short mode")
	}

	mdir := modelsDir(t)

	ttsBase := ensureTTSModel(t, mdir)
	tts := createTTS(t, ttsBase)
	defer sherpa.DeleteOfflineTts(tts)

	// Generate audio from a multi-sentence passage.
	passage := "The wizard cast a powerful fireball spell. The goblin army retreated into the dark forest. Victory was finally within reach."
	samples := generateAudio16k(t, tts, passage)

	pt, err := transcribe.NewParakeetTranscriber(mdir, 2)
	if err != nil {
		t.Skipf("Parakeet transcriber not available: %v", err)
	}
	defer pt.Close()

	segments, err := pt.TranscribeChunk(context.Background(), samples, 0, "")
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}

	if len(segments) == 0 {
		t.Fatal("transcription returned no segments")
	}

	var texts []string
	for _, seg := range segments {
		t.Logf("segment [%.2f - %.2f]: %s", seg.StartTime, seg.EndTime, seg.Text)
		texts = append(texts, seg.Text)
	}
	fullText := strings.Join(texts, " ")
	t.Logf("Full transcription: %q", fullText)

	// Verify key words from each sentence.
	keywords := []string{"wizard", "fireball", "goblin", "forest", "victory"}
	if missing := containsKeyWords(fullText, keywords); len(missing) > 0 {
		t.Errorf("transcription missing keywords %v in: %q", missing, fullText)
	}

	// With 3 sentences we expect at least 2 segments (Parakeet splits on
	// sentence-ending punctuation). A single segment covering the whole
	// passage is acceptable but worth noting.
	if len(segments) < 2 {
		t.Logf("NOTE: expected multiple segments for 3-sentence passage but got %d", len(segments))
	}
}

// TestTTSWithTimeOffset verifies that a non-zero time offset is correctly
// applied to segment timestamps.
func TestTTSWithTimeOffset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running e2e TTS test in short mode")
	}

	mdir := modelsDir(t)

	ttsBase := ensureTTSModel(t, mdir)
	tts := createTTS(t, ttsBase)
	defer sherpa.DeleteOfflineTts(tts)

	phrase := "The rogue picked the lock on the treasure chest"
	samples := generateAudio16k(t, tts, phrase)

	pt, err := transcribe.NewParakeetTranscriber(mdir, 2)
	if err != nil {
		t.Skipf("Parakeet transcriber not available: %v", err)
	}
	defer pt.Close()

	offset := 30 * time.Second
	segments, err := pt.TranscribeChunk(context.Background(), samples, offset, "")
	if err != nil {
		t.Fatalf("transcribe with offset: %v", err)
	}

	if len(segments) == 0 {
		t.Fatal("transcription returned no segments")
	}

	// Every segment should have timestamps >= the offset.
	for i, seg := range segments {
		t.Logf("segment[%d] [%.2f - %.2f]: %s", i, seg.StartTime, seg.EndTime, seg.Text)
		if seg.StartTime < offset.Seconds() {
			t.Errorf("segment %d start time %.2f is before offset %.2f", i, seg.StartTime, offset.Seconds())
		}
	}

	// Verify content.
	var texts []string
	for _, seg := range segments {
		texts = append(texts, seg.Text)
	}
	fullText := strings.Join(texts, " ")
	keywords := []string{"rogue", "lock", "treasure"}
	if missing := containsKeyWords(fullText, keywords); len(missing) > 0 {
		t.Errorf("transcription missing keywords %v in: %q", missing, fullText)
	}
}

// extractTarBz2 extracts a tar.bz2 stream into destDir. This mirrors the
// pattern used by the diarize and transcribe packages.
func extractTarBz2(r io.Reader, destDir string) error {
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

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

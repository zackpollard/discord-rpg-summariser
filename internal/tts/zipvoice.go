package tts

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Synthesizer performs text-to-speech by calling a Python TTS engine as a
// subprocess. Supports multiple engines (zipvoice, f5tts).
type Synthesizer struct {
	projectDir string
	threads    int
	steps      int
	engine     string // "zipvoice" or "f5tts"
	mu         sync.Mutex
	onProgress func(float64)
}

// NewSynthesizer creates a new TTS synthesizer.
// engine should be "zipvoice" or "f5tts".
// It expects a Python venv at {projectDir}/.venv with the engine installed.
func NewSynthesizer(projectDir string, threads int, engine string) (*Synthesizer, error) {
	scriptPath := filepath.Join(projectDir, "scripts", "tts_generate.py")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("TTS script not found at %s", scriptPath)
	}

	venvPython := filepath.Join(projectDir, ".venv", "bin", "python")
	if _, err := os.Stat(venvPython); os.IsNotExist(err) {
		return nil, fmt.Errorf("Python venv not found at %s", venvPython)
	}

	if engine == "" {
		engine = "zipvoice"
	}

	log.Printf("TTS configured: engine=%s (Python subprocess, threads=%d)", engine, threads)
	return &Synthesizer{
		projectDir: projectDir,
		threads:    threads,
		steps:      16,
		engine:     engine,
	}, nil
}

// SetEngine changes the TTS engine.
func (s *Synthesizer) SetEngine(engine string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if engine == "zipvoice" {
		s.engine = engine
		log.Printf("TTS engine switched to %s", engine)
	}
}

// SetProgressCallback sets a function called with progress (0.0-1.0) during generation.
func (s *Synthesizer) SetProgressCallback(fn func(float64)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onProgress = fn
}

// Close is a no-op for the subprocess approach.
func (s *Synthesizer) Close() error {
	return nil
}

// SampleRate returns the output sample rate (ZipVoice outputs 24kHz).
func (s *Synthesizer) SampleRate() int {
	return 24000
}

// Synthesize generates speech from text using voice cloning. It writes a
// reference WAV to a temp file, calls the Python script, and reads the result.
// The context can be used to cancel the generation (kills the subprocess).
func (s *Synthesizer) Synthesize(ctx context.Context, text string, refAudio []float32, refSampleRate int, refText string) ([]float32, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Write reference audio to a temp WAV file.
	refWavPath, err := writeTempWAV(refAudio, refSampleRate)
	if err != nil {
		return nil, 0, fmt.Errorf("write ref wav: %w", err)
	}
	defer os.Remove(refWavPath)

	// Create temp output path.
	outFile, err := os.CreateTemp("", "tts-output-*.wav")
	if err != nil {
		return nil, 0, fmt.Errorf("create temp output: %w", err)
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)

	// Run the Python script.
	scriptPath := filepath.Join(s.projectDir, "scripts", "tts_generate.py")
	venvPython := filepath.Join(s.projectDir, ".venv", "bin", "python")

	cmd := exec.CommandContext(ctx, venvPython, scriptPath,
		"--engine", s.engine,
		"--ref-wav", refWavPath,
		"--ref-text", refText,
		"--text", text,
		"--output", outPath,
		"--steps", strconv.Itoa(s.steps),
		"--threads", strconv.Itoa(s.threads),
	)

	// Parse progress from stderr.
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, 0, fmt.Errorf("stderr pipe: %w", err)
	}

	var stdoutBuf strings.Builder
	cmd.Stdout = &stdoutBuf

	if err := cmd.Start(); err != nil {
		return nil, 0, fmt.Errorf("start tts: %w", err)
	}

	// Read stderr for progress updates.
	go s.parseProgress(stderrPipe)

	if err := cmd.Wait(); err != nil {
		return nil, 0, fmt.Errorf("tts failed: %w\noutput: %s", err, stdoutBuf.String())
	}

	// Read the output WAV.
	samples, err := loadWAV24k(outPath)
	if err != nil {
		return nil, 0, fmt.Errorf("read output wav: %w", err)
	}

	return samples, 24000, nil
}

func (s *Synthesizer) parseProgress(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PROGRESS:") {
			val := strings.TrimPrefix(line, "PROGRESS:")
			if p, err := strconv.ParseFloat(val, 64); err == nil && s.onProgress != nil {
				s.onProgress(p)
			}
		}
	}
}

// writeTempWAV writes float32 samples to a temporary 16-bit mono WAV file.
func writeTempWAV(samples []float32, sampleRate int) (string, error) {
	f, err := os.CreateTemp("", "tts-ref-*.wav")
	if err != nil {
		return "", err
	}
	path := f.Name()
	f.Close()

	if err := WriteWAV(path, samples, sampleRate); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// loadWAV24k reads a 24kHz 16-bit mono WAV file into float32 samples.
func loadWAV24k(path string) ([]float32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 44 {
		return nil, fmt.Errorf("wav too short: %d bytes", len(data))
	}

	pcm := data[44:]
	n := len(pcm) / 2
	samples := make([]float32, n)
	for i := 0; i < n; i++ {
		s := int16(uint16(pcm[i*2]) | uint16(pcm[i*2+1])<<8)
		samples[i] = float32(s) / 32768.0
	}
	return samples, nil
}

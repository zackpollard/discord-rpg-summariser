package audio

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// writeTestWAV creates a minimal 48kHz 16-bit mono WAV file from int16 samples.
func writeTestWAV(t *testing.T, path string, samples []int16) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test wav: %v", err)
	}
	defer f.Close()

	dataSize := uint32(len(samples) * 2)
	var header [44]byte
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], 36+dataSize)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], 1)
	binary.LittleEndian.PutUint32(header[24:28], 48000)
	binary.LittleEndian.PutUint32(header[28:32], 96000)
	binary.LittleEndian.PutUint16(header[32:34], 2)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], dataSize)

	if _, err := f.Write(header[:]); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if err := binary.Write(f, binary.LittleEndian, samples); err != nil {
		t.Fatalf("write samples: %v", err)
	}
}

func TestResampleOutputLength(t *testing.T) {
	// Generate a 1kHz sine wave at 48kHz for 1 second.
	const numSamples = 48000
	samples := make([]int16, numSamples)
	for i := range samples {
		samples[i] = int16(16000 * math.Sin(2*math.Pi*1000*float64(i)/48000))
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "sine.wav")
	writeTestWAV(t, path, samples)

	out, err := LoadAndResample(path)
	if err != nil {
		t.Fatalf("LoadAndResample: %v", err)
	}

	wantLen := numSamples / 3
	if len(out) != wantLen {
		t.Errorf("output length: got %d, want %d", len(out), wantLen)
	}
}

func TestResampleDCPassthrough(t *testing.T) {
	// A DC signal (constant value) should pass through the low-pass filter
	// essentially unchanged (within filter edge effects).
	const numSamples = 48000
	const dcValue int16 = 10000
	samples := make([]int16, numSamples)
	for i := range samples {
		samples[i] = dcValue
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "dc.wav")
	writeTestWAV(t, path, samples)

	out, err := LoadAndResample(path)
	if err != nil {
		t.Fatalf("LoadAndResample: %v", err)
	}

	expected := float32(dcValue) / 32768.0

	// Check the middle portion, avoiding filter edge transients.
	start := len(out) / 4
	end := 3 * len(out) / 4
	for i := start; i < end; i++ {
		if diff := math.Abs(float64(out[i] - expected)); diff > 0.01 {
			t.Errorf("sample[%d]: got %f, want ~%f (diff %f)", i, out[i], expected, diff)
			break
		}
	}
}

func TestResampleFromFile(t *testing.T) {
	// Generate a known signal, write to WAV, resample, and verify basic properties.
	const numSamples = 4800
	samples := make([]int16, numSamples)
	for i := range samples {
		// Low-frequency signal (200Hz) that should survive the 8kHz cutoff.
		samples[i] = int16(8000 * math.Sin(2*math.Pi*200*float64(i)/48000))
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "signal.wav")
	writeTestWAV(t, path, samples)

	out, err := LoadAndResample(path)
	if err != nil {
		t.Fatalf("LoadAndResample: %v", err)
	}

	if len(out) != numSamples/3 {
		t.Errorf("output length: got %d, want %d", len(out), numSamples/3)
	}

	// Verify the output is not all zeros.
	var maxVal float32
	for _, v := range out {
		if v > maxVal {
			maxVal = v
		}
		if -v > maxVal {
			maxVal = -v
		}
	}
	if maxVal < 0.01 {
		t.Error("output signal is too quiet; resampler may have zeroed the signal")
	}
}

func TestResampleChunk_OutputLength(t *testing.T) {
	// 1 second of 48kHz int16 audio should produce 16000 float32 samples.
	const numSamples = 48000
	samples := make([]int16, numSamples)
	for i := range samples {
		samples[i] = int16(16000 * math.Sin(2*math.Pi*1000*float64(i)/48000))
	}

	out := ResampleChunk(samples)

	wantLen := numSamples / 3
	if len(out) != wantLen {
		t.Errorf("ResampleChunk output length: got %d, want %d", len(out), wantLen)
	}
}

func TestResampleChunk_DCPassthrough(t *testing.T) {
	// A constant (DC) input should pass through the low-pass filter unchanged
	// in the middle portion, avoiding edge transients.
	const numSamples = 48000
	const dcValue int16 = 10000
	samples := make([]int16, numSamples)
	for i := range samples {
		samples[i] = dcValue
	}

	out := ResampleChunk(samples)
	expected := float32(dcValue) / 32768.0

	start := len(out) / 4
	end := 3 * len(out) / 4
	for i := start; i < end; i++ {
		if diff := math.Abs(float64(out[i] - expected)); diff > 0.01 {
			t.Errorf("sample[%d]: got %f, want ~%f (diff %f)", i, out[i], expected, diff)
			break
		}
	}
}

func TestResampleChunk_EmptyInput(t *testing.T) {
	out := ResampleChunk(nil)
	if len(out) != 0 {
		t.Errorf("ResampleChunk(nil): got %d samples, want 0", len(out))
	}

	out = ResampleChunk([]int16{})
	if len(out) != 0 {
		t.Errorf("ResampleChunk([]int16{}): got %d samples, want 0", len(out))
	}
}

func TestExtractTimeRange_Normal(t *testing.T) {
	// 3 seconds at 48kHz.
	samples := make([]float32, 3*48000)
	for i := range samples {
		samples[i] = float32(i) / float32(len(samples))
	}

	// Extract 1s-2s.
	got := ExtractTimeRange(samples, 48000, 1.0, 2.0)
	wantLen := 48000
	if len(got) != wantLen {
		t.Errorf("extracted length = %d, want %d", len(got), wantLen)
	}

	// Verify the first sample corresponds to index 48000 in the original.
	if got[0] != samples[48000] {
		t.Errorf("first sample = %f, want %f", got[0], samples[48000])
	}
}

func TestExtractTimeRange_Clamped(t *testing.T) {
	// 1 second of samples.
	samples := make([]float32, 48000)
	for i := range samples {
		samples[i] = 0.5
	}

	// Request 0-5s, should clamp to 0-1s.
	got := ExtractTimeRange(samples, 48000, 0.0, 5.0)
	if len(got) != 48000 {
		t.Errorf("clamped length = %d, want %d", len(got), 48000)
	}
}

func TestExtractTimeRange_Negative(t *testing.T) {
	samples := make([]float32, 48000)
	for i := range samples {
		samples[i] = 0.5
	}

	// Negative start should clamp to 0.
	got := ExtractTimeRange(samples, 48000, -1.0, 0.5)
	wantLen := 24000 // 0.5s at 48kHz
	if len(got) != wantLen {
		t.Errorf("length = %d, want %d", len(got), wantLen)
	}
}

func TestExtractTimeRange_EmptyResult(t *testing.T) {
	samples := make([]float32, 48000)

	// start >= end.
	got := ExtractTimeRange(samples, 48000, 2.0, 1.0)
	if got != nil {
		t.Errorf("expected nil for start >= end, got %d samples", len(got))
	}

	// Both beyond range.
	got = ExtractTimeRange(samples, 48000, 5.0, 6.0)
	if got != nil {
		t.Errorf("expected nil for out-of-range, got %d samples", len(got))
	}
}

func TestLoadRaw48k(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.wav")

	// Create a 0.5s WAV at amplitude 0.5.
	const durSec = 0.5
	const numSamples = int(durSec * 48000)
	samples := make([]int16, numSamples)
	for i := range samples {
		samples[i] = 16384 // ~0.5 amplitude
	}
	writeTestWAV(t, path, samples)

	got, err := LoadRaw48k(path)
	if err != nil {
		t.Fatalf("LoadRaw48k: %v", err)
	}

	if len(got) != numSamples {
		t.Errorf("sample count = %d, want %d", len(got), numSamples)
	}

	// Verify values are approximately 0.5.
	for i := 0; i < 10 && i < len(got); i++ {
		expected := float32(16384) / 32768.0
		diff := got[i] - expected
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.001 {
			t.Errorf("sample[%d] = %f, want ~%f", i, got[i], expected)
		}
	}
}

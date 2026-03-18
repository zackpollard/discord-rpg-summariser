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

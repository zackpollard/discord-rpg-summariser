package audio

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// createTestWAV writes a 48kHz 16-bit mono WAV file filled with a sine wave
// at the given amplitude (0.0–1.0) and duration in seconds.
func createTestWAV(t *testing.T, path string, amplitude float64, durationSec float64) {
	t.Helper()

	numSamples := int(durationSec * mixSampleRate)
	dataSize := uint32(numSamples * 2)

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test wav: %v", err)
	}
	defer f.Close()

	// Write header.
	if err := writeWAVHeader(f, dataSize); err != nil {
		t.Fatalf("write header: %v", err)
	}

	// Write sine wave samples in chunks to keep memory low.
	const chunkSize = 4096
	buf := make([]byte, chunkSize*2)
	freq := 440.0 // 440 Hz tone
	for written := 0; written < numSamples; {
		batch := chunkSize
		if numSamples-written < batch {
			batch = numSamples - written
		}
		for i := 0; i < batch; i++ {
			t := float64(written+i) / float64(mixSampleRate)
			sample := amplitude * math.Sin(2.0*math.Pi*freq*t)
			s := int16(sample * 32767.0)
			binary.LittleEndian.PutUint16(buf[i*2:i*2+2], uint16(s))
		}
		if _, err := f.Write(buf[:batch*2]); err != nil {
			// Use Fatalf with explicit message to avoid shadowing the outer t.
			panic("write samples: " + err.Error())
		}
		written += batch
	}
}

func TestMixAndNormalize(t *testing.T) {
	dir := t.TempDir()

	// Create two WAV files with different amplitudes.
	// User A: loud (amplitude 0.8).
	// User B: quiet (amplitude 0.1).
	pathA := filepath.Join(dir, "user_a.wav")
	pathB := filepath.Join(dir, "user_b.wav")
	createTestWAV(t, pathA, 0.8, 1.0) // 1 second
	createTestWAV(t, pathB, 0.1, 1.0) // 1 second

	outputPath := filepath.Join(dir, "mixed.wav")
	err := MixAndNormalize(map[string]string{
		"a": pathA,
		"b": pathB,
	}, outputPath)
	if err != nil {
		t.Fatalf("MixAndNormalize: %v", err)
	}

	// Verify the output file.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	// Check WAV header basics.
	if len(data) < 44 {
		t.Fatalf("output too short: %d bytes", len(data))
	}
	if string(data[0:4]) != "RIFF" {
		t.Fatalf("missing RIFF header")
	}
	if string(data[8:12]) != "WAVE" {
		t.Fatalf("missing WAVE format")
	}

	sampleRate := binary.LittleEndian.Uint32(data[24:28])
	if sampleRate != mixSampleRate {
		t.Errorf("sample rate = %d, want %d", sampleRate, mixSampleRate)
	}

	bitsPerSample := binary.LittleEndian.Uint16(data[34:36])
	if bitsPerSample != mixBitsPerSamp {
		t.Errorf("bits per sample = %d, want %d", bitsPerSample, mixBitsPerSamp)
	}

	channels := binary.LittleEndian.Uint16(data[22:24])
	if channels != mixNumChannels {
		t.Errorf("channels = %d, want %d", channels, mixNumChannels)
	}

	// Check data size: 1 second at 48kHz = 48000 samples = 96000 bytes.
	dataChunkSize := binary.LittleEndian.Uint32(data[40:44])
	expectedDataSize := uint32(48000 * 2) // 1 second * 48kHz * 2 bytes
	if dataChunkSize != expectedDataSize {
		t.Errorf("data size = %d, want %d", dataChunkSize, expectedDataSize)
	}

	// Verify normalization: scan the output for peak amplitude.
	// Both tracks get normalized so their peaks reach 0.9, then they are
	// summed. The output peak should be close to but not exceed 1.0 (clipping).
	pcm := data[44:]
	numSamples := len(pcm) / 2
	if numSamples != 48000 {
		t.Errorf("output samples = %d, want 48000", numSamples)
	}

	var peak float64
	for i := 0; i < numSamples; i++ {
		s := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		amp := math.Abs(float64(s) / 32768.0)
		if amp > peak {
			peak = amp
		}
	}

	// With both tracks normalised to 0.9 peak and mixed, the output peak
	// should be around 1.8, which gets clipped to 1.0.
	// The important thing is that the quiet track got amplified.
	if peak < 0.5 {
		t.Errorf("output peak = %.3f, expected > 0.5 (normalisation not working)", peak)
	}

	t.Logf("output peak amplitude: %.3f", peak)
}

func TestMixAndNormalize_DifferentLengths(t *testing.T) {
	dir := t.TempDir()

	// User A: 2 seconds, User B: 1 second.
	pathA := filepath.Join(dir, "user_a.wav")
	pathB := filepath.Join(dir, "user_b.wav")
	createTestWAV(t, pathA, 0.5, 2.0)
	createTestWAV(t, pathB, 0.5, 1.0)

	outputPath := filepath.Join(dir, "mixed.wav")
	err := MixAndNormalize(map[string]string{
		"a": pathA,
		"b": pathB,
	}, outputPath)
	if err != nil {
		t.Fatalf("MixAndNormalize: %v", err)
	}

	// Output should be 2 seconds long (the max of the two inputs).
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	dataChunkSize := binary.LittleEndian.Uint32(data[40:44])
	expectedDataSize := uint32(2 * 48000 * 2) // 2 seconds
	if dataChunkSize != expectedDataSize {
		t.Errorf("data size = %d, want %d", dataChunkSize, expectedDataSize)
	}
}

func TestMixAndNormalize_NoFiles(t *testing.T) {
	dir := t.TempDir()
	err := MixAndNormalize(map[string]string{}, filepath.Join(dir, "out.wav"))
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

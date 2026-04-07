package audio

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestMixClip_SingleUser(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "user1.wav")
	createTestWAV(t, wavPath, 0.5, 3.0)

	outPath := filepath.Join(dir, "clip.wav")
	err := MixClip(
		map[string]string{"user1": wavPath},
		outPath,
		map[string]float64{},
		1.0, 2.0, // extract 1s-2s
	)
	if err != nil {
		t.Fatalf("MixClip: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	// 1 second at 48kHz mono 16-bit = 48000 samples = 96000 bytes of PCM.
	dataChunkSize := binary.LittleEndian.Uint32(data[40:44])
	expectedDataSize := uint32(48000 * 2)
	if dataChunkSize != expectedDataSize {
		t.Errorf("data size = %d, want %d", dataChunkSize, expectedDataSize)
	}
}

func TestMixClip_TwoUsers(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.wav")
	pathB := filepath.Join(dir, "b.wav")
	createTestWAV(t, pathA, 0.4, 2.0)
	createTestWAV(t, pathB, 0.4, 2.0)

	outSingle := filepath.Join(dir, "single.wav")
	err := MixClip(
		map[string]string{"a": pathA},
		outSingle,
		map[string]float64{},
		0.0, 1.0,
	)
	if err != nil {
		t.Fatalf("MixClip single: %v", err)
	}

	outBoth := filepath.Join(dir, "both.wav")
	err = MixClip(
		map[string]string{"a": pathA, "b": pathB},
		outBoth,
		map[string]float64{},
		0.0, 1.0,
	)
	if err != nil {
		t.Fatalf("MixClip both: %v", err)
	}

	peakSingle := wavPeakAmplitude(t, outSingle)
	peakBoth := wavPeakAmplitude(t, outBoth)

	t.Logf("single peak: %.3f, both peak: %.3f", peakSingle, peakBoth)
	if peakBoth <= peakSingle {
		t.Errorf("two-user mix (%.3f) should be louder than single user (%.3f)", peakBoth, peakSingle)
	}
}

func TestMixClip_WithJoinOffsets(t *testing.T) {
	dir := t.TempDir()

	// User A starts at 0s with 3s of audio.
	// User B joins at 2s with 2s of audio.
	pathA := filepath.Join(dir, "a.wav")
	pathB := filepath.Join(dir, "b.wav")
	createTestWAV(t, pathA, 0.5, 3.0)
	createTestWAV(t, pathB, 0.5, 2.0)

	outPath := filepath.Join(dir, "clip.wav")
	err := MixClip(
		map[string]string{"a": pathA, "b": pathB},
		outPath,
		map[string]float64{"b": 2.0}, // B joins 2s late
		2.0, 3.0,                     // extract 2s-3s: both users should have audio here
	)
	if err != nil {
		t.Fatalf("MixClip: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	// Should be 1 second of output.
	dataChunkSize := binary.LittleEndian.Uint32(data[40:44])
	expectedDataSize := uint32(48000 * 2)
	if dataChunkSize != expectedDataSize {
		t.Errorf("data size = %d, want %d", dataChunkSize, expectedDataSize)
	}

	// Verify there is actual audio content (not silence).
	peak := wavPeakAmplitude(t, outPath)
	if peak < 0.1 {
		t.Errorf("peak amplitude %.3f is too low, expected audible content", peak)
	}
}

func TestMixClip_NoOverlap(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "user1.wav")
	createTestWAV(t, wavPath, 0.5, 1.0) // 1 second of audio starting at offset 0

	outPath := filepath.Join(dir, "clip.wav")
	// Request time range 5-6s, but user only has audio from 0-1s.
	err := MixClip(
		map[string]string{"user1": wavPath},
		outPath,
		map[string]float64{},
		5.0, 6.0,
	)
	if err == nil {
		t.Fatal("expected error for no overlap, got nil")
	}
}

func TestMixClip_InvalidRange(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "user1.wav")
	createTestWAV(t, wavPath, 0.5, 1.0)

	outPath := filepath.Join(dir, "clip.wav")
	// End before start.
	err := MixClip(
		map[string]string{"user1": wavPath},
		outPath,
		map[string]float64{},
		2.0, 1.0,
	)
	if err == nil {
		t.Fatal("expected error for invalid range, got nil")
	}
}

// wavPeakAmplitude reads a WAV file and returns the peak amplitude as a float64 in [0, 1].
func wavPeakAmplitude(t *testing.T, path string) float64 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read wav: %v", err)
	}
	if len(data) < 44 {
		t.Fatalf("wav too short")
	}
	pcm := data[44:]
	numSamples := len(pcm) / 2
	var peak float64
	for i := 0; i < numSamples; i++ {
		s := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		amp := math.Abs(float64(s) / 32768.0)
		if amp > peak {
			peak = amp
		}
	}
	return peak
}

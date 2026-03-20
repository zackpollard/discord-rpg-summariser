package audio

import (
	"math"
	"path/filepath"
	"testing"
)

// genSine48 generates 48kHz 16-bit mono sine wave samples of the given
// duration and frequency.
func genSine48(durationSec float64, freqHz float64, amplitude int16) []int16 {
	n := int(48000 * durationSec)
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = int16(float64(amplitude) * math.Sin(2*math.Pi*freqHz*float64(i)/48000))
	}
	return samples
}

// genSilence48 generates 48kHz silent samples of the given duration.
func genSilence48(durationSec float64) []int16 {
	n := int(48000 * durationSec)
	return make([]int16, n)
}

func TestStreamResampleAll_OutputEquivalence(t *testing.T) {
	// Generate a 5-second 200Hz sine wave — a frequency that survives the
	// 8kHz low-pass cutoff — and verify StreamResampleAll matches
	// LoadAndResample within tolerance.
	samples := genSine48(5.0, 200, 8000)

	dir := t.TempDir()
	path := filepath.Join(dir, "equiv.wav")
	writeTestWAV(t, path, samples)

	reference, err := LoadAndResample(path)
	if err != nil {
		t.Fatalf("LoadAndResample: %v", err)
	}

	streamed, err := StreamResampleAll(path)
	if err != nil {
		t.Fatalf("StreamResampleAll: %v", err)
	}

	// The streamed output uses a stateful FIR which produces slightly
	// different edge behaviour from the zero-padded batch filter. Compare
	// length within a small tolerance and values in the middle portion.
	if diff := len(reference) - len(streamed); diff < -1 || diff > 1 {
		t.Fatalf("length mismatch: reference=%d streamed=%d", len(reference), len(streamed))
	}

	// The batch filter centres the kernel (reads ahead kLen/2) and zero-pads,
	// while the streaming filter is causal (ring buffer). This introduces a
	// constant delay of kLen/2 output samples and different edge transients.
	// To compare, we align the two outputs by shifting the streamed signal
	// back by half the kernel length, then compare the middle 50%.
	shift := (filterTaps + 1) / 2 / decimationFactor // kernel centre in output samples
	minLen := len(reference)
	if len(streamed)-shift < minLen {
		minLen = len(streamed) - shift
	}
	start := minLen / 4
	end := 3 * minLen / 4
	var maxDiff float64
	for i := start; i < end; i++ {
		d := math.Abs(float64(reference[i] - streamed[i+shift]))
		if d > maxDiff {
			maxDiff = d
		}
	}
	// After alignment the values should be very close in the middle.
	if maxDiff > 0.05 {
		t.Errorf("max sample difference in middle 50%% (shift=%d): %f (want < 0.05)", shift, maxDiff)
	}
}

func TestStreamResample_ChunkSplitOnSilence(t *testing.T) {
	// Build a WAV: 10s tone, 2s silence, 10s tone.
	// The silence boundary should split into 2 chunks. The first chunk
	// must be at least 30s for a silence split, so with only 10s of tone
	// the silence won't trigger early — but the second tone extends the
	// first chunk past 30s. We actually need chunks >= 30s, so we use
	// 32s + 2s silence + 10s to ensure the split can happen.
	var samples []int16
	samples = append(samples, genSine48(32.0, 300, 10000)...)
	samples = append(samples, genSilence48(2.0)...)
	samples = append(samples, genSine48(10.0, 300, 10000)...)

	dir := t.TempDir()
	path := filepath.Join(dir, "silence_split.wav")
	writeTestWAV(t, path, samples)

	type chunk struct {
		numSamples int
		offset     float64
	}
	var chunks []chunk

	err := StreamResample(path, func(s []float32, off float64) error {
		chunks = append(chunks, chunk{numSamples: len(s), offset: off})
		return nil
	})
	if err != nil {
		t.Fatalf("StreamResample: %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	// First chunk starts at offset 0.
	if chunks[0].offset != 0 {
		t.Errorf("chunk 0 offset: got %f, want 0", chunks[0].offset)
	}
	// Second chunk should start somewhere around 32-34 seconds.
	if chunks[1].offset < 30 || chunks[1].offset > 36 {
		t.Errorf("chunk 1 offset: got %f, want ~32-34", chunks[1].offset)
	}
}

func TestStreamResample_MaxChunkEnforcement(t *testing.T) {
	// Generate 6 minutes (360s) of continuous tone — no silence at all.
	// Should be forcibly split into two chunks: 5 min + 1 min.
	samples := genSine48(360, 300, 10000)

	dir := t.TempDir()
	path := filepath.Join(dir, "max_chunk.wav")
	writeTestWAV(t, path, samples)

	type chunk struct {
		numSamples int
		offset     float64
	}
	var chunks []chunk

	err := StreamResample(path, func(s []float32, off float64) error {
		chunks = append(chunks, chunk{numSamples: len(s), offset: off})
		return nil
	})
	if err != nil {
		t.Fatalf("StreamResample: %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	// First chunk should be exactly maxChunkSamples.
	if chunks[0].numSamples != maxChunkSamples {
		t.Errorf("chunk 0 samples: got %d, want %d", chunks[0].numSamples, maxChunkSamples)
	}
	if chunks[0].offset != 0 {
		t.Errorf("chunk 0 offset: got %f, want 0", chunks[0].offset)
	}
	// Second chunk should have the remainder.
	if chunks[1].numSamples <= 0 {
		t.Error("chunk 1 should have samples")
	}
}

func TestStreamResample_MinChunkEnforcement(t *testing.T) {
	// Generate audio with brief silences (0.5s) every 10 seconds over a
	// 45-second span. The brief silences should NOT cause premature splits
	// because chunks must be >= 30s.
	var samples []int16
	for i := 0; i < 4; i++ {
		samples = append(samples, genSine48(10.0, 400, 10000)...)
		samples = append(samples, genSilence48(0.5)...)
	}
	samples = append(samples, genSine48(3.0, 400, 10000)...) // tail

	dir := t.TempDir()
	path := filepath.Join(dir, "min_chunk.wav")
	writeTestWAV(t, path, samples)

	type chunk struct {
		numSamples int
		offset     float64
	}
	var chunks []chunk

	err := StreamResample(path, func(s []float32, off float64) error {
		chunks = append(chunks, chunk{numSamples: len(s), offset: off})
		return nil
	})
	if err != nil {
		t.Fatalf("StreamResample: %v", err)
	}

	// All chunks should be >= minChunkSamples, except possibly the last one.
	for i, c := range chunks {
		if i < len(chunks)-1 && c.numSamples < minChunkSamples {
			t.Errorf("chunk %d has %d samples (< min %d)", i, c.numSamples, minChunkSamples)
		}
	}
}

func TestStreamResample_ShortFile(t *testing.T) {
	// A 5-second file should produce exactly 1 chunk.
	samples := genSine48(5.0, 440, 8000)

	dir := t.TempDir()
	path := filepath.Join(dir, "short.wav")
	writeTestWAV(t, path, samples)

	var chunkCount int
	err := StreamResample(path, func(_ []float32, _ float64) error {
		chunkCount++
		return nil
	})
	if err != nil {
		t.Fatalf("StreamResample: %v", err)
	}

	if chunkCount != 1 {
		t.Errorf("expected 1 chunk for short file, got %d", chunkCount)
	}
}

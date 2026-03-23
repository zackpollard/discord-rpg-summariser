package voice

import (
	"testing"
	"time"
)

// collectChunks drains the channel and returns all chunks received before
// the channel is idle for a short timeout.
func collectChunks(t *testing.T, ch <-chan ChunkReady) []ChunkReady {
	t.Helper()
	var got []ChunkReady
	for {
		select {
		case c := <-ch:
			got = append(got, c)
		case <-time.After(10 * time.Millisecond):
			return got
		}
	}
}

// constantPCM returns n samples all set to val.
func constantPCM(val int16, n int) []int16 {
	pcm := make([]int16, n)
	for i := range pcm {
		pcm[i] = val
	}
	return pcm
}

// silentPCM returns n zero-valued samples.
func silentPCM(n int) []int16 {
	return make([]int16, n)
}

func TestLiveBufferFlushesAtStride(t *testing.T) {
	ch := make(chan ChunkReady, 16)
	lb := NewLiveBuffer("user1", "Thordak", time.Now(), 0, ch)

	// Add exactly strideSamples of loud audio. This should trigger a flush.
	lb.AddSamples(constantPCM(5000, strideSamples))

	chunks := collectChunks(t, ch)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	if chunks[0].UserID != "user1" {
		t.Errorf("UserID: got %q, want %q", chunks[0].UserID, "user1")
	}
	if chunks[0].DisplayName != "Thordak" {
		t.Errorf("DisplayName: got %q, want %q", chunks[0].DisplayName, "Thordak")
	}
	if len(chunks[0].Samples) != strideSamples {
		t.Errorf("chunk sample count: got %d, want %d", len(chunks[0].Samples), strideSamples)
	}
}

func TestLiveBufferOverlap(t *testing.T) {
	ch := make(chan ChunkReady, 16)
	lb := NewLiveBuffer("user1", "Thordak", time.Now(), 0, ch)

	// First flush: add strideSamples to trigger.
	lb.AddSamples(constantPCM(5000, strideSamples))
	collectChunks(t, ch) // drain first chunk

	// After flush the internal buffer should contain overlapSamples.
	// Adding strideSamples more should produce a chunk of overlap+stride = windowSamples.
	lb.AddSamples(constantPCM(5000, strideSamples))

	chunks := collectChunks(t, ch)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk after second stride, got %d", len(chunks))
	}

	wantLen := overlapSamples + strideSamples
	if len(chunks[0].Samples) != wantLen {
		t.Errorf("second chunk sample count: got %d, want %d (overlap %d + stride %d)",
			len(chunks[0].Samples), wantLen, overlapSamples, strideSamples)
	}
}

func TestLiveBufferSilenceFlush(t *testing.T) {
	ch := make(chan ChunkReady, 16)
	lb := NewLiveBuffer("user1", "Thordak", time.Now(), 0, ch)

	// Add minFlushSamples of loud audio (not enough for stride).
	lb.AddSamples(constantPCM(5000, minFlushSamples))

	// No flush yet because we haven't reached stride or silence threshold.
	chunks := collectChunks(t, ch)
	if len(chunks) != 0 {
		t.Fatalf("expected no chunks before silence, got %d", len(chunks))
	}

	// Feed enough silence frames to trigger the silence threshold.
	// Each frame is frameSamples (960 samples at 48kHz = 20ms).
	for i := 0; i < silenceFrames; i++ {
		lb.AddSamples(silentPCM(frameSamples))
	}

	chunks = collectChunks(t, ch)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk after silence, got %d", len(chunks))
	}
}

func TestLiveBufferMinFlush(t *testing.T) {
	ch := make(chan ChunkReady, 16)
	lb := NewLiveBuffer("user1", "Thordak", time.Now(), 0, ch)

	// Add less than minFlushSamples of audio, then silence.
	lb.AddSamples(constantPCM(5000, minFlushSamples-frameSamples))

	for i := 0; i < silenceFrames; i++ {
		lb.AddSamples(silentPCM(frameSamples))
	}

	// Even with silence, buffer shouldn't flush because new audio < minFlushSamples.
	// (The silence frames themselves count as new audio and push it over, so we need
	// to ensure the loud audio alone is under the threshold.)
	// Actually silence frames *do* add new samples. Check: loud + silence frames.
	// loud = minFlushSamples - frameSamples = 96000 - 960 = 95040
	// silence = silenceFrames * frameSamples = 40 * 960 = 38400
	// total new = 95040 + 38400 = 133440 > minFlushSamples, so it will flush.
	// To properly test "no flush under min", we need total new < minFlushSamples.
	chunks := collectChunks(t, ch)

	// Reset and test with truly insufficient audio.
	ch2 := make(chan ChunkReady, 16)
	lb2 := NewLiveBuffer("user2", "Bard", time.Now(), 0, ch2)

	// Add only a tiny amount of audio, well under minFlushSamples.
	lb2.AddSamples(constantPCM(5000, frameSamples))

	// Feed silence — total new = frameSamples + silenceFrames*frameSamples
	// = 960 + 38400 = 39360 < 96000 = minFlushSamples
	for i := 0; i < silenceFrames; i++ {
		lb2.AddSamples(silentPCM(frameSamples))
	}

	chunks = collectChunks(t, ch2)
	if len(chunks) != 0 {
		t.Errorf("expected no flush when new samples (%d) < minFlushSamples (%d), got %d chunks",
			frameSamples+silenceFrames*frameSamples, minFlushSamples, len(chunks))
	}
}

func TestLiveBufferFlushOnClose(t *testing.T) {
	ch := make(chan ChunkReady, 16)
	lb := NewLiveBuffer("user1", "Thordak", time.Now(), 0, ch)

	// Add enough audio for Flush() to emit (>= minFlushSamples).
	lb.AddSamples(constantPCM(5000, minFlushSamples))

	// No automatic flush should have happened (under strideSamples).
	chunks := collectChunks(t, ch)
	if len(chunks) != 0 {
		t.Fatalf("expected no auto-flush, got %d chunks", len(chunks))
	}

	// Explicit Flush() should send remaining audio.
	lb.Flush()

	chunks = collectChunks(t, ch)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk after Flush(), got %d", len(chunks))
	}
	if len(chunks[0].Samples) != minFlushSamples {
		t.Errorf("flushed sample count: got %d, want %d", len(chunks[0].Samples), minFlushSamples)
	}
}

func TestIsSilent(t *testing.T) {
	// Zero samples are silent.
	if !isSilent(silentPCM(960)) {
		t.Error("expected zero samples to be silent")
	}

	// Empty slice is silent.
	if !isSilent([]int16{}) {
		t.Error("expected empty slice to be silent")
	}

	// Loud samples are not silent.
	if isSilent(constantPCM(5000, 960)) {
		t.Error("expected loud samples to not be silent")
	}

	// Very quiet but nonzero samples below the threshold are still silent.
	if !isSilent(constantPCM(10, 960)) {
		t.Error("expected very quiet samples to be silent")
	}
}

func TestLiveBufferChunkSeq(t *testing.T) {
	ch := make(chan ChunkReady, 16)
	lb := NewLiveBuffer("user1", "Thordak", time.Now(), 0, ch)

	// Trigger three flushes.
	for i := 0; i < 3; i++ {
		lb.AddSamples(constantPCM(5000, strideSamples))
	}

	chunks := collectChunks(t, ch)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}

	for i, c := range chunks {
		wantSeq := i + 1
		if c.ChunkSeq != wantSeq {
			t.Errorf("chunk %d: ChunkSeq got %d, want %d", i, c.ChunkSeq, wantSeq)
		}
	}
}

func TestLiveBufferStartOffset(t *testing.T) {
	ch := make(chan ChunkReady, 16)
	lb := NewLiveBuffer("user1", "Thordak", time.Now(), 0, ch)

	// First flush: strideSamples of new audio, no overlap yet.
	lb.AddSamples(constantPCM(5000, strideSamples))
	chunks := collectChunks(t, ch)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	// First chunk starts at offset 0 (no previous audio).
	if chunks[0].StartOffset != 0 {
		t.Errorf("first chunk offset: got %v, want 0", chunks[0].StartOffset)
	}

	// Second flush: overlap carries forward, new audio starts at strideSamples.
	lb.AddSamples(constantPCM(5000, strideSamples))
	chunks = collectChunks(t, ch)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	// Second chunk's new audio starts at strideSamples, but the chunk includes
	// overlapSamples of earlier audio, so the chunk start offset should be:
	// totalNew (strideSamples) / 48000 seconds - overlap duration
	overlapDur := time.Duration(overlapSamples) * time.Second / 48000
	wantOffset := time.Duration(strideSamples)*time.Second/48000 - overlapDur
	if wantOffset < 0 {
		wantOffset = 0
	}

	if chunks[0].StartOffset != wantOffset {
		t.Errorf("second chunk offset: got %v, want %v", chunks[0].StartOffset, wantOffset)
	}
}

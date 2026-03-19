package voice

import (
	"log"
	"math"
	"time"
)

const (
	chunkDuration   = 10 * time.Second
	chunkSamples    = 48000 * 10 // 10s at 48kHz
	minFlushSamples = 48000 * 2  // don't flush less than 2s
	silenceFrames   = 75         // ~1.5s of 20ms frames
	silenceRMSThresh = 50        // int16 RMS below this = silence
)

// ChunkReady is emitted when a live buffer has enough audio for transcription.
type ChunkReady struct {
	UserID      string
	DisplayName string
	Samples     []int16       // 48kHz mono PCM
	StartOffset time.Duration // offset from session start
}

// LiveBuffer accumulates decoded PCM per user and emits chunks for live
// transcription when full or after silence is detected.
type LiveBuffer struct {
	userID       string
	displayName  string
	buf          []int16
	silenceCount int
	totalSamples int64
	sessionStart time.Time
	out          chan<- ChunkReady
}

func NewLiveBuffer(userID, displayName string, sessionStart time.Time, out chan<- ChunkReady) *LiveBuffer {
	return &LiveBuffer{
		userID:       userID,
		displayName:  displayName,
		buf:          make([]int16, 0, chunkSamples),
		sessionStart: sessionStart,
		out:          out,
	}
}

// AddSamples appends decoded PCM and flushes if a chunk is ready.
func (lb *LiveBuffer) AddSamples(pcm []int16) {
	lb.buf = append(lb.buf, pcm...)

	if isSilent(pcm) {
		lb.silenceCount++
	} else {
		lb.silenceCount = 0
	}

	// Flush on silence boundary (if we have enough audio) or max chunk size
	if lb.silenceCount >= silenceFrames && len(lb.buf) >= minFlushSamples {
		lb.flush()
	} else if len(lb.buf) >= chunkSamples {
		lb.flush()
	}
}

// Flush sends any buffered audio as a chunk.
func (lb *LiveBuffer) Flush() {
	if len(lb.buf) >= minFlushSamples {
		lb.flush()
	}
}

func (lb *LiveBuffer) flush() {
	chunk := make([]int16, len(lb.buf))
	copy(chunk, lb.buf)

	offset := time.Duration(lb.totalSamples) * time.Second / 48000
	log.Printf("LiveBuffer flushing %.1fs of audio for %s at offset %v",
		float64(len(chunk))/48000.0, lb.userID, offset)

	select {
	case lb.out <- ChunkReady{
		UserID:      lb.userID,
		DisplayName: lb.displayName,
		Samples:     chunk,
		StartOffset: offset,
	}:
	default:
		// drop chunk if channel is full — better than blocking the audio path
	}

	lb.totalSamples += int64(len(lb.buf))
	lb.buf = lb.buf[:0]
	lb.silenceCount = 0
}

func isSilent(pcm []int16) bool {
	if len(pcm) == 0 {
		return true
	}
	var sumSq float64
	for _, s := range pcm {
		sumSq += float64(s) * float64(s)
	}
	rms := math.Sqrt(sumSq / float64(len(pcm)))
	return rms < silenceRMSThresh
}

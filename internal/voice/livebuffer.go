package voice

import (
	"log"
	"math"
	"time"
)

const (
	windowDuration   = 5 * time.Second               // total audio sent to whisper per chunk
	windowSamples    = 48000 * 5                     // 5s at 48kHz
	strideDuration   = 3 * time.Second               // new audio between chunks
	strideSamples    = 48000 * 3                     // 3s at 48kHz
	overlapSamples   = windowSamples - strideSamples // 2s kept for context
	minFlushSamples  = 48000 * 2                     // don't flush less than 2s
	silenceFrames    = 40                            // ~0.8s of 20ms frames
	silenceRMSThresh = 50
)

// ChunkReady is emitted when a live buffer has enough audio for transcription.
type ChunkReady struct {
	UserID      string
	DisplayName string
	Samples     []int16       // 48kHz mono PCM (windowDuration of audio)
	StartOffset time.Duration // session-relative offset of this chunk's start
	ChunkSeq    int           // monotonically increasing sequence number
}

// LiveBuffer accumulates decoded PCM per user and emits overlapping chunks
// for live transcription. Each chunk contains windowDuration of audio, with
// overlapSamples carried over from the previous chunk for whisper context.
type LiveBuffer struct {
	userID       string
	displayName  string
	buf          []int16
	overlap      []int16 // last overlapSamples from previous chunk
	silenceCount int
	totalNew     int64 // total NEW samples (excluding overlap)
	sessionStart time.Time
	chunkSeq     int
	out          chan<- ChunkReady
}

func NewLiveBuffer(userID, displayName string, sessionStart time.Time, out chan<- ChunkReady) *LiveBuffer {
	return &LiveBuffer{
		userID:       userID,
		displayName:  displayName,
		sessionStart: sessionStart,
		buf:          make([]int16, 0, windowSamples),
		out:          out,
	}
}

// AddSamples appends decoded PCM and flushes when stride is reached.
func (lb *LiveBuffer) AddSamples(pcm []int16) {
	lb.buf = append(lb.buf, pcm...)

	if isSilent(pcm) {
		lb.silenceCount++
	} else {
		lb.silenceCount = 0
	}

	newSamples := len(lb.buf) - len(lb.overlap)

	// Flush on silence boundary (natural sentence break) or stride reached
	if lb.silenceCount >= silenceFrames && newSamples >= minFlushSamples {
		lb.flush()
	} else if newSamples >= strideSamples {
		lb.flush()
	}
}

// Flush sends any remaining buffered audio.
func (lb *LiveBuffer) Flush() {
	newSamples := len(lb.buf) - len(lb.overlap)
	if newSamples >= minFlushSamples {
		lb.flush()
	}
}

func (lb *LiveBuffer) flush() {
	// The chunk to send is the full buffer (overlap + new audio)
	chunk := make([]int16, len(lb.buf))
	copy(chunk, lb.buf)

	// Offset is based on where this chunk starts in the session
	// The overlap part started earlier, so subtract overlap duration
	newBeforeThis := lb.totalNew
	overlapDur := time.Duration(len(lb.overlap)) * time.Second / 48000
	offset := time.Duration(newBeforeThis)*time.Second/48000 - overlapDur
	if offset < 0 {
		offset = 0
	}

	lb.chunkSeq++
	log.Printf("LiveBuffer flushing %.1fs for %s (seq=%d, offset=%v, %d new + %d overlap samples)",
		float64(len(chunk))/48000.0, lb.userID, lb.chunkSeq, offset,
		len(lb.buf)-len(lb.overlap), len(lb.overlap))

	select {
	case lb.out <- ChunkReady{
		UserID:      lb.userID,
		DisplayName: lb.displayName,
		Samples:     chunk,
		StartOffset: offset,
		ChunkSeq:    lb.chunkSeq,
	}:
	default:
		// drop if channel full
	}

	// Track how many new samples we've processed
	lb.totalNew += int64(len(lb.buf) - len(lb.overlap))

	// Keep the last overlapSamples for the next chunk's context
	if len(lb.buf) > overlapSamples {
		lb.overlap = make([]int16, overlapSamples)
		copy(lb.overlap, lb.buf[len(lb.buf)-overlapSamples:])
	} else {
		lb.overlap = make([]int16, len(lb.buf))
		copy(lb.overlap, lb.buf)
	}

	lb.buf = make([]int16, 0, windowSamples)
	lb.buf = append(lb.buf, lb.overlap...)
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

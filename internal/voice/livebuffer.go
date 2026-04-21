package voice

import (
	"log"
	"math"
	"sync"
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
	// staleFlushDuration is how long to wait before flushing pending buffered
	// audio whose stride hasn't been reached (e.g. a user paused mid-sentence
	// and Discord stopped sending packets). Keeps live transcript responsive.
	staleFlushDuration = 4 * time.Second
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
	mu           sync.Mutex
	userID       string
	displayName  string
	buf          []int16
	overlap      []int16 // last overlapSamples from previous chunk
	silenceCount int
	totalNew     int64 // total NEW samples (excluding overlap)
	sessionStart time.Time
	joinOffset   time.Duration // offset from session start to this user's first audio
	chunkSeq     int
	lastAddAt    time.Time // wall clock of the last AddSamples call
	out          chan<- ChunkReady
	stop         chan struct{}
}

func NewLiveBuffer(userID, displayName string, sessionStart time.Time, joinOffset time.Duration, out chan<- ChunkReady) *LiveBuffer {
	lb := &LiveBuffer{
		userID:       userID,
		displayName:  displayName,
		sessionStart: sessionStart,
		joinOffset:   joinOffset,
		buf:          make([]int16, 0, windowSamples),
		out:          out,
		stop:         make(chan struct{}),
	}
	go lb.staleFlusher()
	return lb
}

// Close stops the background stale-flush goroutine. Safe to call multiple times.
func (lb *LiveBuffer) Close() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	select {
	case <-lb.stop:
		// already closed
	default:
		close(lb.stop)
	}
}

// staleFlusher periodically checks whether buffered audio should be flushed
// due to inactivity (Discord stops sending packets when a user is silent, so
// AddSamples alone can't trigger a flush in that case).
func (lb *LiveBuffer) staleFlusher() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-lb.stop:
			return
		case <-ticker.C:
			lb.mu.Lock()
			if !lb.lastAddAt.IsZero() && time.Since(lb.lastAddAt) >= staleFlushDuration {
				newSamples := len(lb.buf) - len(lb.overlap)
				if newSamples >= minFlushSamples {
					lb.flush()
				}
			}
			lb.mu.Unlock()
		}
	}
}

// AddSamples appends decoded PCM and flushes when stride is reached.
func (lb *LiveBuffer) AddSamples(pcm []int16) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.buf = append(lb.buf, pcm...)
	lb.lastAddAt = time.Now()

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
	lb.mu.Lock()
	defer lb.mu.Unlock()
	newSamples := len(lb.buf) - len(lb.overlap)
	if newSamples >= minFlushSamples {
		lb.flush()
	}
}

func (lb *LiveBuffer) flush() {
	// Trim trailing silence (from RTP-gap fill) so the transcriber doesn't
	// waste CPU cycles on empty audio. Only trim the tail; the head may
	// contain meaningful silence-then-speech that the model needs for context.
	trimmed := trimTrailingSilence(lb.buf)

	// If after trimming there's nothing left beyond the overlap, skip
	// sending to the transcriber entirely.
	newSamples := len(trimmed) - len(lb.overlap)
	if newSamples < minFlushSamples {
		// Still advance state so the overlap tracks the original buf
		// (we DO want raw audio for subsequent context).
		lb.totalNew += int64(len(lb.buf) - len(lb.overlap))
		if len(lb.buf) > overlapSamples {
			lb.overlap = make([]int16, overlapSamples)
			copy(lb.overlap, lb.buf[len(lb.buf)-overlapSamples:])
		} else {
			lb.overlap = make([]int16, len(lb.buf))
			copy(lb.overlap, lb.buf)
		}
		lb.buf = lb.buf[:0]
		lb.buf = append(lb.buf, lb.overlap...)
		lb.silenceCount = 0
		lb.lastAddAt = time.Time{}
		return
	}

	chunk := make([]int16, len(trimmed))
	copy(chunk, trimmed)

	// Offset is based on where this chunk starts in the session.
	// Add the user's join offset so timestamps are session-relative.
	newBeforeThis := lb.totalNew
	overlapDur := time.Duration(len(lb.overlap)) * time.Second / 48000
	offset := lb.joinOffset + time.Duration(newBeforeThis)*time.Second/48000 - overlapDur
	if offset < 0 {
		offset = 0
	}

	lb.chunkSeq++
	trimmedSamples := len(lb.buf) - len(trimmed)
	log.Printf("LiveBuffer flushing %.1fs for %s (seq=%d, offset=%v, %d new + %d overlap, trimmed %d trailing silence samples)",
		float64(len(chunk))/48000.0, lb.userID, lb.chunkSeq, offset,
		len(trimmed)-len(lb.overlap), len(lb.overlap), trimmedSamples)

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
	lb.lastAddAt = time.Time{}
}

// trimTrailingSilence returns a prefix of pcm ending at the last non-silent
// frame. Walks backwards in ~20ms frames (960 samples at 48kHz) and trims
// every frame that's below the silence RMS threshold. Returns the original
// slice if no trailing silence is detected.
func trimTrailingSilence(pcm []int16) []int16 {
	const frame = 960 // 20ms @ 48kHz
	if len(pcm) < frame {
		return pcm
	}
	end := len(pcm)
	for end >= frame {
		start := end - frame
		if !isSilent(pcm[start:end]) {
			break
		}
		end = start
	}
	return pcm[:end]
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

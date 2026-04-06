package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

// ChunkCallback is called for each silence-delimited chunk of resampled audio.
// samples contains 16kHz float32 mono PCM. offsetSeconds is the time offset
// of this chunk's first sample relative to the start of the file.
type ChunkCallback func(samples []float32, offsetSeconds float64) error

const (
	// silenceWindowSamples is the number of 16kHz samples in one silence
	// detection frame (100ms).
	silenceWindowSamples = 1600

	// silenceRMSThreshold is the float32-scale RMS below which a frame is
	// considered silent (~= int16 RMS 98/32768).
	silenceRMSThreshold = 0.003

	// silenceMinFrames is the number of consecutive silent frames required
	// before a split is allowed (10 frames = 1 second).
	silenceMinFrames = 10

	// minChunkSamples is the minimum number of 16kHz output samples a chunk
	// must contain before a silence-based split is allowed (15 seconds).
	minChunkSamples = 15 * outputRate

	// maxChunkSamples is the hard limit at which a chunk is forcibly split
	// even without silence (90 seconds). Shorter chunks reduce peak memory
	// usage during ONNX inference.
	maxChunkSamples = 90 * outputRate

	// streamReadBytes is the number of bytes to read per iteration from the
	// WAV file (1 second of 48kHz 16-bit mono PCM).
	streamReadBytes = inputRate * 2
)

// filterState holds the ring buffer for stateful FIR filtering across blocks.
type filterState struct {
	kernel []float64
	ring   []float32 // ring buffer of length len(kernel)
	pos    int       // next write position in ring
}

func newFilterState(kernel []float64) *filterState {
	return &filterState{
		kernel: kernel,
		ring:   make([]float32, len(kernel)),
		pos:    0,
	}
}

// process pushes input samples through the FIR filter and returns the
// filtered output. The ring buffer carries state between calls.
func (fs *filterState) process(input []float32) []float32 {
	out := make([]float32, len(input))
	kLen := len(fs.kernel)

	for i, s := range input {
		// Write sample into the ring buffer.
		fs.ring[fs.pos] = s

		// Convolve: walk the kernel, reading the ring backwards from pos.
		var acc float64
		idx := fs.pos
		for j := 0; j < kLen; j++ {
			acc += float64(fs.ring[idx]) * fs.kernel[j]
			idx--
			if idx < 0 {
				idx = kLen - 1
			}
		}
		out[i] = float32(acc)

		fs.pos++
		if fs.pos >= kLen {
			fs.pos = 0
		}
	}
	return out
}

// decimateState tracks the decimation phase across blocks so that the 3:1
// pick pattern is seamless.
type decimateState struct {
	phase int // number of input samples seen mod decimationFactor
}

// process decimates filtered samples, returning only every decimationFactor-th
// sample relative to the running phase.
func (ds *decimateState) process(input []float32) []float32 {
	var out []float32
	for _, s := range input {
		if ds.phase == 0 {
			out = append(out, s)
		}
		ds.phase++
		if ds.phase >= decimationFactor {
			ds.phase = 0
		}
	}
	return out
}

// StreamResample opens a 48kHz 16-bit mono WAV file and streams resampled
// 16kHz audio in chunks split at silence boundaries. Each chunk is passed
// to the callback with its time offset in seconds.
func StreamResample(wavPath string, cb ChunkCallback) error {
	f, err := os.Open(wavPath)
	if err != nil {
		return fmt.Errorf("open wav: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat wav: %w", err)
	}
	if info.Size() < wavHeaderSkip {
		return fmt.Errorf("wav file too short: %d bytes", info.Size())
	}

	// Skip the 44-byte WAV header.
	if _, err := f.Seek(wavHeaderSkip, 0); err != nil {
		return fmt.Errorf("seek past header: %w", err)
	}

	kernel := buildSincKernel(filterTaps, cutoff)
	fState := newFilterState(kernel)
	dState := &decimateState{}

	readBuf := make([]byte, streamReadBytes)

	// Chunk accumulation state.
	var chunkBuf []float32
	var silenceFrameCount int        // consecutive silent frames
	var totalInputSamples int64      // cumulative input samples read (for offset tracking)
	var chunkStartInputSamples int64 // input sample index where the current chunk started

	// flush delivers the current chunk and resets accumulation state.
	flush := func() error {
		if len(chunkBuf) == 0 {
			return nil
		}
		offsetSeconds := float64(chunkStartInputSamples) / float64(inputRate)
		err := cb(chunkBuf, offsetSeconds)
		chunkBuf = nil
		silenceFrameCount = 0
		chunkStartInputSamples = totalInputSamples
		return err
	}

	// silenceCheckBuf accumulates output samples for RMS checking in
	// silenceWindowSamples-sized frames.
	var silenceCheckBuf []float32

	for {
		n, readErr := f.Read(readBuf)
		if n == 0 && readErr != nil {
			break
		}

		// Convert the bytes we read into int16, then float32.
		nSamples := n / 2
		floats := make([]float32, nSamples)
		for i := 0; i < nSamples; i++ {
			s := int16(binary.LittleEndian.Uint16(readBuf[i*2 : i*2+2]))
			floats[i] = float32(s) / 32768.0
		}

		// Filter and decimate.
		filtered := fState.process(floats)
		resampled := dState.process(filtered)

		totalInputSamples += int64(nSamples)

		// Append to the silence check buffer and process frames.
		silenceCheckBuf = append(silenceCheckBuf, resampled...)

		for len(silenceCheckBuf) >= silenceWindowSamples {
			frame := silenceCheckBuf[:silenceWindowSamples]
			silenceCheckBuf = silenceCheckBuf[silenceWindowSamples:]

			chunkBuf = append(chunkBuf, frame...)

			if isFrameSilent(frame) {
				silenceFrameCount++
			} else {
				silenceFrameCount = 0
			}

			// Check split conditions.
			chunkLen := len(chunkBuf)

			// Forced split at max chunk size.
			if chunkLen >= maxChunkSamples {
				if err := flush(); err != nil {
					return err
				}
				continue
			}

			// Silence-based split: enough consecutive silence AND chunk is
			// at least the minimum size.
			if silenceFrameCount >= silenceMinFrames && chunkLen >= minChunkSamples {
				if err := flush(); err != nil {
					return err
				}
			}
		}

		if readErr != nil {
			break
		}
	}

	// Flush any remaining samples from the silence check buffer.
	if len(silenceCheckBuf) > 0 {
		chunkBuf = append(chunkBuf, silenceCheckBuf...)
		silenceCheckBuf = nil
	}

	// Deliver the final chunk.
	return flush()
}

// isFrameSilent computes the RMS of a float32 frame and returns true if it
// falls below the silence threshold.
func isFrameSilent(frame []float32) bool {
	if len(frame) == 0 {
		return true
	}
	var sumSq float64
	for _, s := range frame {
		sumSq += float64(s) * float64(s)
	}
	rms := math.Sqrt(sumSq / float64(len(frame)))
	return rms < silenceRMSThreshold
}

// StreamResampleAll collects all streamed chunks into one contiguous slice.
// This is a convenience wrapper for callers that need the full output at once.
func StreamResampleAll(wavPath string) ([]float32, error) {
	var all []float32
	err := StreamResample(wavPath, func(samples []float32, _ float64) error {
		all = append(all, samples...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

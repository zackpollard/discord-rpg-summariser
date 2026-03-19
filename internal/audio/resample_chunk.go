package audio

import "sync"

var (
	chunkKernel     []float64
	chunkKernelOnce sync.Once
)

// ResampleChunk takes 48kHz int16 mono PCM samples and returns 16kHz float32
// samples suitable for whisper. Uses the same sinc/blackman FIR filter as
// LoadAndResample but operates on in-memory buffers.
func ResampleChunk(samples []int16) []float32 {
	chunkKernelOnce.Do(func() {
		chunkKernel = buildSincKernel(filterTaps, cutoff)
	})

	floats := make([]float32, len(samples))
	for i, s := range samples {
		floats[i] = float32(s) / 32768.0
	}

	filtered := applyFilter(floats, chunkKernel)
	return decimate(filtered, decimationFactor)
}

package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

const (
	inputRate        = 48000
	outputRate       = 16000
	decimationFactor = inputRate / outputRate // 3
	filterTaps       = 32
	wavHeaderSkip    = 44
	cutoff           = float64(outputRate/2) / float64(inputRate) // 8000/48000
)

// LoadAndResample reads a 48kHz 16-bit mono WAV file and returns float32
// samples resampled to 16kHz, suitable for Whisper.
func LoadAndResample(wavPath string) ([]float32, error) {
	data, err := os.ReadFile(wavPath)
	if err != nil {
		return nil, fmt.Errorf("read wav file: %w", err)
	}
	if len(data) < wavHeaderSkip {
		return nil, fmt.Errorf("wav file too short: %d bytes", len(data))
	}

	pcmData := data[wavHeaderSkip:]
	numSamples := len(pcmData) / 2
	if numSamples == 0 {
		return nil, nil
	}

	// Convert int16 PCM to float32.
	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		s := int16(binary.LittleEndian.Uint16(pcmData[i*2 : i*2+2]))
		samples[i] = float32(s) / 32768.0
	}

	// Build the FIR low-pass filter kernel.
	kernel := buildSincKernel(filterTaps, cutoff)

	// Apply filter and decimate 3:1.
	filtered := applyFilter(samples, kernel)
	output := decimate(filtered, decimationFactor)

	return output, nil
}

// buildSincKernel generates a windowed sinc low-pass FIR filter.
// taps is the filter length (even), fc is the normalised cutoff (0..0.5).
func buildSincKernel(taps int, fc float64) []float64 {
	kernel := make([]float64, taps+1)
	mid := float64(taps) / 2.0
	sum := 0.0

	for i := 0; i <= taps; i++ {
		x := float64(i) - mid
		// Sinc function.
		var sinc float64
		if x == 0 {
			sinc = 2.0 * math.Pi * fc
		} else {
			sinc = math.Sin(2.0*math.Pi*fc*x) / x
		}
		// Blackman window.
		w := 0.42 - 0.5*math.Cos(2.0*math.Pi*float64(i)/float64(taps)) +
			0.08*math.Cos(4.0*math.Pi*float64(i)/float64(taps))
		kernel[i] = sinc * w
		sum += kernel[i]
	}

	// Normalise so the filter has unity gain at DC.
	for i := range kernel {
		kernel[i] /= sum
	}
	return kernel
}

// applyFilter convolves the input signal with the FIR kernel.
func applyFilter(samples []float32, kernel []float64) []float32 {
	n := len(samples)
	kLen := len(kernel)
	out := make([]float32, n)

	for i := 0; i < n; i++ {
		var acc float64
		for j := 0; j < kLen; j++ {
			idx := i - j + kLen/2
			if idx >= 0 && idx < n {
				acc += float64(samples[idx]) * kernel[j]
			}
		}
		out[i] = float32(acc)
	}
	return out
}

// decimate picks every factor-th sample from the input.
func decimate(samples []float32, factor int) []float32 {
	outLen := len(samples) / factor
	out := make([]float32, outLen)
	for i := 0; i < outLen; i++ {
		out[i] = samples[i*factor]
	}
	return out
}

// LoadRaw48k reads a 48kHz 16-bit mono WAV file into float32 samples
// without resampling. Useful when you need the original sample rate preserved.
func LoadRaw48k(wavPath string) ([]float32, error) {
	data, err := os.ReadFile(wavPath)
	if err != nil {
		return nil, fmt.Errorf("read wav file: %w", err)
	}
	if len(data) < wavHeaderSkip {
		return nil, fmt.Errorf("wav file too short: %d bytes", len(data))
	}

	pcmData := data[wavHeaderSkip:]
	numSamples := len(pcmData) / 2
	if numSamples == 0 {
		return nil, nil
	}

	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		s := int16(binary.LittleEndian.Uint16(pcmData[i*2 : i*2+2]))
		samples[i] = float32(s) / 32768.0
	}

	return samples, nil
}

// ExtractTimeRange returns the slice of samples between startSec and endSec
// at the given sample rate. Out-of-bounds indices are clamped.
func ExtractTimeRange(samples []float32, sampleRate int, startSec, endSec float64) []float32 {
	startIdx := int(startSec * float64(sampleRate))
	endIdx := int(endSec * float64(sampleRate))

	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(samples) {
		endIdx = len(samples)
	}
	if startIdx >= endIdx {
		return nil
	}

	return samples[startIdx:endIdx]
}

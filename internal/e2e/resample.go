package e2e

// resampleLinear resamples float32 audio from srcRate to dstRate using linear
// interpolation. Quality is sufficient for test audio where we only need
// intelligible speech for transcription verification.
func resampleLinear(samples []float32, srcRate, dstRate int) []float32 {
	if srcRate == dstRate {
		return samples
	}

	ratio := float64(srcRate) / float64(dstRate)
	outLen := int(float64(len(samples)) / ratio)
	if outLen == 0 {
		return nil
	}

	out := make([]float32, outLen)
	for i := range out {
		srcIdx := float64(i) * ratio
		idx0 := int(srcIdx)
		frac := float32(srcIdx - float64(idx0))

		if idx0+1 < len(samples) {
			out[i] = samples[idx0]*(1-frac) + samples[idx0+1]*frac
		} else if idx0 < len(samples) {
			out[i] = samples[idx0]
		}
	}

	return out
}

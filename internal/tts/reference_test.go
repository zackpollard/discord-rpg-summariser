package tts

import (
	"math"
	"strings"
	"testing"
)

func TestScoreClip_SpeechDensity(t *testing.T) {
	// Generate samples with moderate RMS so energy doesn't dominate scoring.
	samples := makeSineFloat32(8.0, 48000, 0.05)

	// 2-3 wps over 8s = 16-24 words. Use 20 words (2.5 wps).
	goodText := strings.Repeat("word ", 20)
	goodScore, _ := scoreClip(samples, 8.0, goodText)

	// Very slow: 0.5 wps over 8s = 4 words.
	slowText := "one two three four"
	slowScore, _ := scoreClip(samples, 8.0, slowText)

	t.Logf("good density score: %.3f, slow density score: %.3f", goodScore, slowScore)
	if goodScore <= slowScore {
		t.Errorf("natural pace (%.3f) should score higher than slow pace (%.3f)", goodScore, slowScore)
	}
}

func TestScoreClip_RMSEnergy(t *testing.T) {
	dur := 8.0
	text := strings.Repeat("word ", 20) // 2.5 wps, good density

	// Near-silence.
	silentSamples := make([]float32, int(dur*48000))
	for i := range silentSamples {
		silentSamples[i] = 0.0001
	}
	silentScore, _ := scoreClip(silentSamples, dur, text)

	// Normal speech level (RMS ~0.05).
	normalSamples := makeSineFloat32(dur, 48000, 0.07)
	normalScore, _ := scoreClip(normalSamples, dur, text)

	// Clipping level (RMS ~0.5).
	clipSamples := makeSineFloat32(dur, 48000, 0.7)
	clipScore, _ := scoreClip(clipSamples, dur, text)

	t.Logf("silent: %.3f, normal: %.3f, clipping: %.3f", silentScore, normalScore, clipScore)
	if silentScore >= normalScore {
		t.Errorf("silence (%.3f) should score lower than normal speech (%.3f)", silentScore, normalScore)
	}
	if clipScore >= normalScore {
		t.Errorf("clipping (%.3f) should score lower than normal speech (%.3f)", clipScore, normalScore)
	}
}

func TestScoreClip_Duration(t *testing.T) {
	text := strings.Repeat("word ", 20) // keep wps reasonable across durations

	// Ideal duration (8s).
	idealSamples := makeSineFloat32(8.0, 48000, 0.05)
	idealScore, _ := scoreClip(idealSamples, 8.0, text)

	// Short duration (4s) -- adjust words to keep wps ~2.5.
	shortText := strings.Repeat("word ", 10)
	shortSamples := makeSineFloat32(4.0, 48000, 0.05)
	shortScore, _ := scoreClip(shortSamples, 4.0, shortText)

	t.Logf("ideal dur score: %.3f, short dur score: %.3f", idealScore, shortScore)
	if idealScore <= shortScore {
		t.Errorf("ideal duration (%.3f) should score higher than short (%.3f)", idealScore, shortScore)
	}
}

func TestCleanReferenceText_MidSentenceStart(t *testing.T) {
	input := "ent through the dark forest"
	result := cleanReferenceText(input)
	// "ent" is lowercase, should be trimmed. Next words are also lowercase,
	// so it drops just the first word.
	if strings.HasPrefix(result, "ent") {
		t.Errorf("expected leading fragment to be trimmed, got %q", result)
	}
	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestCleanReferenceText_CleanText(t *testing.T) {
	input := "The party arrived at dawn."
	result := cleanReferenceText(input)
	if result != input {
		t.Errorf("clean text should be unchanged: got %q, want %q", result, input)
	}
}

func TestCleanReferenceText_AllLowercase(t *testing.T) {
	input := "going through the cave"
	result := cleanReferenceText(input)
	// First word is lowercase, no capital found nearby, drops first word.
	if strings.HasPrefix(result, "going") {
		t.Errorf("expected first lowercase word to be dropped, got %q", result)
	}
	if result != "through the cave" {
		t.Errorf("got %q, want %q", result, "through the cave")
	}
}

func TestCleanReferenceText_Empty(t *testing.T) {
	result := cleanReferenceText("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// makeSineFloat32 generates a sine wave at 440Hz with the given amplitude
// and duration at the specified sample rate.
func makeSineFloat32(durSec float64, sampleRate int, amplitude float64) []float32 {
	n := int(durSec * float64(sampleRate))
	samples := make([]float32, n)
	freq := 440.0
	for i := 0; i < n; i++ {
		t := float64(i) / float64(sampleRate)
		samples[i] = float32(amplitude * math.Sin(2.0*math.Pi*freq*t))
	}
	return samples
}

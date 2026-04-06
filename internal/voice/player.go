package voice

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/hraban/opus.v2"

	"github.com/bwmarrin/discordgo"
)

const (
	discordSampleRate = 48000
	discordChannels   = 1
	frameSize         = 960 // 20ms at 48kHz
	frameDuration     = 20 * time.Millisecond
)

// PlayWAV reads a WAV file and streams it as Opus audio through a Discord
// voice connection. The WAV file can be any sample rate — it will be
// resampled to 48kHz for Discord.
func PlayWAV(vc *discordgo.VoiceConnection, wavPath string) error {
	samples, srcRate, err := loadWAVSamples(wavPath)
	if err != nil {
		return fmt.Errorf("load wav: %w", err)
	}

	// Resample to 48kHz if needed.
	if srcRate != discordSampleRate {
		samples = resampleLinear(samples, srcRate, discordSampleRate)
	}

	// Convert float32 to int16 PCM for the Opus encoder.
	pcm := make([]int16, len(samples))
	for i, s := range samples {
		if s > 1.0 {
			s = 1.0
		} else if s < -1.0 {
			s = -1.0
		}
		pcm[i] = int16(s * 32767)
	}

	// Create Opus encoder.
	encoder, err := opus.NewEncoder(discordSampleRate, discordChannels, opus.AppVoip)
	if err != nil {
		return fmt.Errorf("create opus encoder: %w", err)
	}

	// Signal that we're speaking.
	if err := vc.Speaking(true); err != nil {
		return fmt.Errorf("speaking: %w", err)
	}
	defer vc.Speaking(false)

	// Send frames at 20ms intervals.
	opusBuf := make([]byte, 4000)
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	totalFrames := len(pcm) / frameSize
	for i := 0; i+frameSize <= len(pcm); i += frameSize {
		frame := pcm[i : i+frameSize]

		n, err := encoder.Encode(frame, opusBuf)
		if err != nil {
			log.Printf("voice player: opus encode error: %v", err)
			continue
		}

		select {
		case vc.OpusSend <- opusBuf[:n]:
		case <-time.After(time.Second):
			return fmt.Errorf("opus send timed out at frame %d/%d", i/frameSize, totalFrames)
		}

		<-ticker.C
	}

	// Brief silence at the end so Discord doesn't cut off the last frame.
	time.Sleep(250 * time.Millisecond)

	return nil
}

// loadWAVSamples reads a WAV file and returns float32 samples and the sample rate.
func loadWAVSamples(path string) ([]float32, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	if len(data) < 44 {
		return nil, 0, fmt.Errorf("wav too short: %d bytes", len(data))
	}

	sampleRate := int(binary.LittleEndian.Uint32(data[24:28]))
	pcmData := data[44:]
	n := len(pcmData) / 2
	samples := make([]float32, n)
	for i := 0; i < n; i++ {
		s := int16(binary.LittleEndian.Uint16(pcmData[i*2 : i*2+2]))
		samples[i] = float32(s) / 32768.0
	}

	return samples, sampleRate, nil
}

// resampleLinear performs simple linear interpolation resampling.
// Good enough for upsampling 24kHz→48kHz (just doubles each sample
// with linear interpolation, which is fine for speech).
func resampleLinear(samples []float32, srcRate, dstRate int) []float32 {
	ratio := float64(srcRate) / float64(dstRate)
	outLen := int(float64(len(samples)) / ratio)
	out := make([]float32, outLen)

	for i := range out {
		srcPos := float64(i) * ratio
		idx := int(srcPos)
		frac := float32(srcPos - float64(idx))

		if idx+1 < len(samples) {
			out[i] = samples[idx]*(1-frac) + samples[idx+1]*frac
		} else if idx < len(samples) {
			out[i] = samples[idx]
		}
	}

	return out
}

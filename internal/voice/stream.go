package voice

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/hraban/opus.v2"
)

const (
	streamSampleRate  = 48000
	streamChannels    = 2 // Discord sends stereo opus
	frameSamples      = 960 // 20ms at 48kHz
	maxSilenceSamples = 240000 // 5 seconds at 48kHz
	pcmBufSize        = 5760 * streamChannels // max frame size at 48kHz (120ms) * channels
)

// UserStream manages a single user's audio recording.
type UserStream struct {
	userID        string
	wav           *WAVWriter
	decoder       *opus.Decoder
	lastTimestamp uint32
	sampleRate    int
	hasFirstPkt   bool
	pktCount      int
}

// NewUserStream creates a WAV writer and initialises an opus decoder for the given user.
func NewUserStream(userID, outputDir string) (*UserStream, error) {
	path := filepath.Join(outputDir, userID+".wav")

	w, err := NewWAVWriter(path)
	if err != nil {
		return nil, fmt.Errorf("create wav writer for user %s: %w", userID, err)
	}

	// Discord sends stereo opus; decode to stereo then downmix to mono for WAV.
	dec, err := opus.NewDecoder(streamSampleRate, streamChannels)
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("create opus decoder for user %s: %w", userID, err)
	}

	return &UserStream{
		userID:     userID,
		wav:        w,
		decoder:    dec,
		sampleRate: streamSampleRate,
	}, nil
}

// stripDAVEFrame checks if data is a DAVE secure frame (ends with 0xFA 0xFA)
// and strips the wrapper to extract the inner opus data. For bot passthrough
// frames the "ciphertext" is actually plaintext opus.
func stripDAVEFrame(data []byte) []byte {
	if len(data) < 4 {
		return data
	}
	// Check for DAVE magic trailer
	if data[len(data)-1] != 0xFA || data[len(data)-2] != 0xFA {
		return data // not a DAVE frame, use as-is
	}
	// supplementalSize byte is at len-3
	supplementalSize := int(data[len(data)-3])
	if supplementalSize >= len(data) || supplementalSize < 3 {
		return data // invalid, use as-is
	}
	// The opus data (plaintext for passthrough) is everything before the supplemental area
	opusEnd := len(data) - supplementalSize
	if opusEnd <= 0 {
		return data
	}
	return data[:opusEnd]
}

// HandlePacket decodes an Opus packet to PCM, inserts silence for RTP timestamp
// gaps, and writes the resulting samples to the WAV file.
func (us *UserStream) HandlePacket(packet *discordgo.Packet) error {
	opusData := packet.Opus
	us.pktCount++

	// Log first few packets for debugging
	if us.pktCount <= 3 {
		log.Printf("Packet #%d for user %s: %d bytes, first bytes: %x",
			us.pktCount, us.userID, len(opusData), firstN(opusData, 16))
		if len(opusData) > 4 {
			log.Printf("  last 4 bytes: %x", opusData[len(opusData)-4:])
		}
	}

	// Strip DAVE secure frame wrapper if present
	stripped := stripDAVEFrame(opusData)
	if len(stripped) != len(opusData) && us.pktCount <= 3 {
		log.Printf("  Stripped DAVE frame: %d -> %d bytes", len(opusData), len(stripped))
	}
	opusData = stripped

	pcm := make([]int16, pcmBufSize)
	n, err := us.decoder.Decode(opusData, pcm)
	if err != nil {
		// If stereo decode fails, try the raw data without stripping
		if len(stripped) != len(packet.Opus) {
			n, err = us.decoder.Decode(packet.Opus, pcm)
		}
		if err != nil {
			return fmt.Errorf("decode opus (%d bytes): %w", len(opusData), err)
		}
	}
	pcm = pcm[:n*streamChannels]

	// Downmix stereo to mono for WAV output
	mono := make([]int16, n)
	for i := 0; i < n; i++ {
		l := int32(pcm[i*2])
		r := int32(pcm[i*2+1])
		mono[i] = int16((l + r) / 2)
	}

	if us.hasFirstPkt {
		expected := us.lastTimestamp + uint32(frameSamples)
		if packet.Timestamp > expected {
			gap := int(packet.Timestamp - expected)
			if gap > maxSilenceSamples {
				gap = maxSilenceSamples
			}
			silence := make([]int16, gap)
			if err := us.wav.Write(silence); err != nil {
				return fmt.Errorf("write silence: %w", err)
			}
		}
	}
	us.hasFirstPkt = true
	us.lastTimestamp = packet.Timestamp

	if err := us.wav.Write(mono); err != nil {
		return fmt.Errorf("write pcm: %w", err)
	}
	return nil
}

// Close shuts down the opus decoder and closes the WAV writer.
func (us *UserStream) Close() error {
	return us.wav.Close()
}

// FilePath returns the path of the underlying WAV file.
func (us *UserStream) FilePath() string {
	return us.wav.file.Name()
}

func firstN(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}

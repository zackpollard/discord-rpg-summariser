package voice

import (
	"fmt"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/hraban/opus.v2"
)

const (
	streamSampleRate   = 48000
	streamChannels     = 1
	frameSamples       = 960 // 20ms at 48kHz
	maxSilenceSamples  = 240000 // 5 seconds at 48kHz
	pcmBufSize         = 5760   // max frame size at 48kHz (120ms)
)

// UserStream manages a single user's audio recording.
type UserStream struct {
	userID        string
	wav           *WAVWriter
	decoder       *opus.Decoder
	lastTimestamp uint32
	sampleRate    int
	hasFirstPkt   bool
}

// NewUserStream creates a WAV writer and initialises an opus decoder for the given user.
func NewUserStream(userID, outputDir string) (*UserStream, error) {
	path := filepath.Join(outputDir, userID+".wav")

	w, err := NewWAVWriter(path)
	if err != nil {
		return nil, fmt.Errorf("create wav writer for user %s: %w", userID, err)
	}

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

// HandlePacket decodes an Opus packet to PCM, inserts silence for RTP timestamp
// gaps, and writes the resulting samples to the WAV file.
func (us *UserStream) HandlePacket(packet *discordgo.Packet) error {
	pcm := make([]int16, pcmBufSize)
	n, err := us.decoder.Decode(packet.Opus, pcm)
	if err != nil {
		return fmt.Errorf("decode opus: %w", err)
	}
	pcm = pcm[:n]

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

	if err := us.wav.Write(pcm); err != nil {
		return fmt.Errorf("write pcm: %w", err)
	}
	return nil
}

// Close shuts down the opus decoder and closes the WAV writer.
func (us *UserStream) Close() error {
	// opus.Decoder does not require explicit close; just close the WAV file.
	return us.wav.Close()
}

// FilePath returns the path of the underlying WAV file.
func (us *UserStream) FilePath() string {
	return us.wav.file.Name()
}

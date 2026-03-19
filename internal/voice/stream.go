package voice

import (
	"fmt"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/hraban/opus.v2"
)

const (
	streamSampleRate  = 48000
	streamChannels    = 2 // Discord sends stereo opus
	frameSamples      = 960 // 20ms at 48kHz
	maxSilenceSamples = 240000 // 5 seconds at 48kHz
	pcmBufSize        = 5760 * streamChannels
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
	daveState     *discordgo.ReceiverState
}

// NewUserStream creates a WAV writer and initialises an opus decoder for the given user.
func NewUserStream(userID, outputDir string, daveState *discordgo.ReceiverState) (*UserStream, error) {
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
		daveState:  daveState,
	}, nil
}

// parseDAVEFrame checks if data ends with 0xFAFA and returns the raw frame
// for decryption. Returns nil if not a DAVE frame.
func isDAVEFrame(data []byte) bool {
	return len(data) >= 13 && data[len(data)-1] == 0xFA && data[len(data)-2] == 0xFA
}

// HandlePacket decodes an Opus packet to PCM, inserts silence for RTP timestamp
// gaps, and writes the resulting samples to the WAV file.
func (us *UserStream) HandlePacket(packet *discordgo.Packet) error {
	opusData := packet.Opus
	us.pktCount++

	// Decrypt DAVE secure frame if present
	if isDAVEFrame(opusData) {
		if us.daveState != nil {
			decrypted, err := discordgo.DecryptFrame(us.daveState, opusData)
			if err != nil {
				return fmt.Errorf("dave decrypt (%d bytes): %w", len(opusData), err)
			}
			opusData = decrypted
		} else {
			return nil // no key yet, skip
		}
	} else if len(opusData) < 10 {
		return nil // skip small pre-transition packets
	}
	// Non-DAVE frames > 10 bytes that don't end with 0xFAFA are pre-transition;
	// pass through to opus decoder (may fail, which is fine — they're transient).

	pcm := make([]int16, pcmBufSize)
	n, err := us.decoder.Decode(opusData, pcm)
	if err != nil {
		return fmt.Errorf("decode opus (%d bytes): %w", len(opusData), err)
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


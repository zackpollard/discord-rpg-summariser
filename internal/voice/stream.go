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

	if us.pktCount <= 5 && len(opusData) > 5 {
		last := opusData[len(opusData)-min(8, len(opusData)):]
		log.Printf("Packet #%d for user %s: %d bytes, first: %x, last: %x",
			us.pktCount, us.userID, len(opusData), firstN(opusData, 8), last)
	}

	// Decrypt DAVE secure frame if present
	if isDAVEFrame(opusData) {
		if us.daveState != nil {
			decrypted, err := discordgo.DecryptFrame(us.daveState, opusData)
			if err != nil {
				if us.pktCount <= 200 {
					log.Printf("  DAVE decrypt failed (pkt %d, %d bytes): %v", us.pktCount, len(opusData), err)
				}
				return fmt.Errorf("dave decrypt (%d bytes): %w", len(opusData), err)
			}
			if us.pktCount <= 200 {
				log.Printf("  DAVE decrypted pkt %d: %d -> %d bytes, first: %x",
					us.pktCount, len(opusData), len(decrypted), firstN(decrypted, 4))
			}
			opusData = decrypted
		} else if us.pktCount <= 200 {
			log.Printf("  DAVE frame detected (pkt %d, %d bytes) but no receiver key", us.pktCount, len(opusData))
		}
	} else if us.pktCount <= 200 && len(opusData) > 5 {
		log.Printf("  Not a DAVE frame (pkt %d, %d bytes), last 4: %x", us.pktCount, len(opusData), opusData[len(opusData)-4:])
	}

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

func firstN(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}

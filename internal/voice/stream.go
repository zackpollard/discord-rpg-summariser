package voice

import (
	"fmt"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/hraban/opus.v2"
)

const (
	streamSampleRate  = 48000
	streamChannels    = 1 // Discord voice sends mono opus (TOC byte 0x78 = CELT FB 20ms mono)
	frameSamples      = 960 // 20ms at 48kHz
	maxSilenceSamples = 240000 // 5 seconds at 48kHz
	pcmBufSize        = 5760 * streamChannels
)

// UserStream manages a single user's audio recording.
type UserStream struct {
	userID     string
	wav        *WAVWriter
	decoder    *opus.Decoder
	lastTS     uint32
	hasFirstTS bool
	daveState  *discordgo.ReceiverState
	daveActive bool // true once we've successfully decrypted at least one frame
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
		userID:    userID,
		wav:       w,
		decoder:   dec,
		daveState: daveState,
	}, nil
}

func isDAVEFrame(data []byte) bool {
	if len(data) < 13 || data[len(data)-1] != 0xFA || data[len(data)-2] != 0xFA {
		return false
	}
	ss := int(data[len(data)-3])
	return ss >= 12 && ss < len(data)
}

// HandlePacket decodes an Opus packet to PCM, inserts silence for RTP timestamp
// gaps, and writes the resulting samples to the WAV file.
func (us *UserStream) HandlePacket(packet *discordgo.Packet) error {
	opusData := packet.Opus

	// --- DAVE decryption ---
	if isDAVEFrame(opusData) {
		if us.daveState == nil {
			return nil // no key, skip
		}
		decrypted, err := discordgo.DecryptFrame(us.daveState, opusData)
		if err != nil {
			// Decryption failed — use PLC (packet loss concealment) so the
			// decoder stays in sync rather than getting garbage.
			us.decodePLC(packet.Timestamp)
			return nil
		}
		opusData = decrypted
		us.daveActive = true
	} else if us.daveActive {
		// DAVE is active but this packet isn't a DAVE frame — it's either a
		// pre-transition straggler or a false negative. Never feed encrypted
		// data to the opus decoder; use PLC instead.
		us.decodePLC(packet.Timestamp)
		return nil
	} else {
		// DAVE not yet active. Skip everything until we get the first
		// successful DAVE decrypt to avoid corrupting decoder state.
		return nil
	}

	// --- Opus decode ---
	pcm := make([]int16, pcmBufSize)
	n, err := us.decoder.Decode(opusData, pcm)
	if err != nil {
		return fmt.Errorf("decode opus (%d bytes): %w", len(opusData), err)
	}
	pcm = pcm[:n]

	// --- Silence gap insertion ---
	if us.hasFirstTS {
		expected := us.lastTS + uint32(frameSamples)
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
	us.hasFirstTS = true
	us.lastTS = packet.Timestamp

	return us.wav.Write(pcm)
}

// decodePLC runs opus Packet Loss Concealment for a missing frame, writing
// the interpolated audio to the WAV file. This keeps the decoder state clean
// so subsequent real frames decode without artifacts.
func (us *UserStream) decodePLC(timestamp uint32) {
	if !us.daveActive {
		return // don't PLC before decoder has any real state
	}
	pcm := make([]int16, frameSamples)
	n, err := us.decoder.Decode(nil, pcm)
	if err != nil || n == 0 {
		return
	}
	pcm = pcm[:n]

	if us.hasFirstTS {
		expected := us.lastTS + uint32(frameSamples)
		if timestamp > expected {
			gap := int(timestamp - expected)
			if gap > maxSilenceSamples {
				gap = maxSilenceSamples
			}
			silence := make([]int16, gap)
			us.wav.Write(silence)
		}
	}
	us.hasFirstTS = true
	us.lastTS = timestamp

	us.wav.Write(pcm)
}

// Close closes the WAV writer.
func (us *UserStream) Close() error {
	return us.wav.Close()
}

// FilePath returns the path of the underlying WAV file.
func (us *UserStream) FilePath() string {
	return us.wav.file.Name()
}

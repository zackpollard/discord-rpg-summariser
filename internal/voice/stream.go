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

// findDAVEFrame scans the last few bytes of data for the 0xFAFA DAVE trailer.
// The RTP extension stripping can occasionally leave extra trailing bytes,
// so we scan rather than only checking the final 2 positions.
// Returns the trimmed DAVE frame and true, or the original data and false.
func findDAVEFrame(data []byte) ([]byte, bool) {
	if len(data) < 13 {
		return data, false
	}
	// Scan the last 8 bytes looking for 0xFA 0xFA with a valid supplementalSize.
	limit := min(8, len(data)-12)
	for offset := 0; offset < limit; offset++ {
		pos := len(data) - 2 - offset
		if data[pos] == 0xFA && data[pos+1] == 0xFA {
			ss := int(data[pos-1])
			frameLen := pos + 2
			ciphertextLen := frameLen - ss
			if ss >= 12 && ciphertextLen >= 1 {
				return data[:frameLen], true
			}
		}
	}
	return data, false
}

// isOpusSilence checks if a packet is a raw (unencrypted) opus silence/DTX frame.
// Discord sends these even during active DAVE sessions.
func isOpusSilence(data []byte) bool {
	return len(data) <= 3 && len(data) > 0 && data[0] == 0xF8
}

// HandlePacket decodes an Opus packet to PCM, inserts silence for RTP timestamp
// gaps, and writes the resulting samples to the WAV file.
func (us *UserStream) HandlePacket(packet *discordgo.Packet) error {
	opusData := packet.Opus

	// --- DAVE decryption ---
	daveFrame, isDave := findDAVEFrame(opusData)
	if isDave {
		if us.daveState == nil {
			return nil // no key, skip
		}
		decrypted, err := discordgo.DecryptFrame(us.daveState, daveFrame)
		if err != nil {
			log.Printf("DAVE decrypt failed for %s (seq=%d, %d bytes): %v",
				us.userID, packet.Sequence, len(daveFrame), err)
			us.decodePLC(packet.Timestamp)
			return nil
		}
		opusData = decrypted
		us.daveActive = true
	} else if us.daveActive {
		if isOpusSilence(opusData) {
			// Raw unencrypted silence frame from Discord — decode directly
			// to keep decoder state clean.
		} else {
			last4 := opusData[max(0, len(opusData)-4):]
			log.Printf("Lost packet for %s (seq=%d, %d bytes, last4=%x) — no DAVE trailer found, using PLC",
				us.userID, packet.Sequence, len(opusData), last4)
			us.decodePLC(packet.Timestamp)
			return nil
		}
	} else {
		return nil // pre-DAVE, skip
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

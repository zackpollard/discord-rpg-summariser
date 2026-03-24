package voice

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/hraban/opus.v2"
)

const (
	sampleRate      = 48000
	channels        = 1    // Discord voice sends mono opus
	frameSamples    = 960  // 20ms at 48kHz
	maxPCMFrameSize = 5760 // max opus frame at 48kHz (120ms)
)

// UserStream manages a single user's audio recording: DAVE decryption,
// opus decoding, and WAV writing.
type UserStream struct {
	userID     string
	wav        *WAVWriter
	decoder    *opus.Decoder
	lastTS     uint32
	hasFirstTS bool
	daveState  *discordgo.ReceiverState
	daveActive bool // true after the first successful DAVE decrypt
	liveBuf    *LiveBuffer
}

// NewUserStream creates a WAV writer and opus decoder for the given user.
func NewUserStream(userID, outputDir string, daveState *discordgo.ReceiverState) (*UserStream, error) {
	path := filepath.Join(outputDir, userID+".wav")

	w, err := NewWAVWriter(path)
	if err != nil {
		return nil, fmt.Errorf("create wav writer for user %s: %w", userID, err)
	}

	dec, err := opus.NewDecoder(sampleRate, channels)
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

// findDAVEFrame scans the last few bytes for the 0xFAFA DAVE secure frame
// trailer. RTP extension stripping can leave extra trailing bytes, so we scan
// up to 8 positions rather than checking only the final two bytes.
func findDAVEFrame(data []byte) ([]byte, bool) {
	if len(data) < 13 {
		return data, false
	}
	limit := min(8, len(data)-12)
	for offset := 0; offset < limit; offset++ {
		pos := len(data) - 2 - offset
		if data[pos] == 0xFA && data[pos+1] == 0xFA {
			ss := int(data[pos-1])
			frameLen := pos + 2
			if ss >= 12 && frameLen-ss >= 1 {
				return data[:frameLen], true
			}
		}
	}
	return data, false
}

// isOpusSilence returns true for raw (unencrypted) opus DTX silence frames.
// Discord sends these outside of DAVE encryption even during active sessions.
func isOpusSilence(data []byte) bool {
	return len(data) > 0 && len(data) <= 3 && data[0] == 0xF8
}

// HandlePacket decrypts, decodes, and writes a single voice packet to the WAV file.
func (us *UserStream) HandlePacket(packet *discordgo.Packet) error {
	opusData := packet.Opus

	daveFrame, isDave := findDAVEFrame(opusData)
	if isDave {
		if us.daveState == nil {
			return nil
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
			// Falls through to normal opus decode below.
		} else {
			log.Printf("Lost packet for %s (seq=%d, %d bytes, last4=%x)",
				us.userID, packet.Sequence, len(opusData), opusData[max(0, len(opusData)-4):])
			us.decodePLC(packet.Timestamp)
			return nil
		}
	} else {
		return nil // skip everything before DAVE is active
	}

	pcm := make([]int16, maxPCMFrameSize)
	n, err := us.decoder.Decode(opusData, pcm)
	if err != nil {
		return fmt.Errorf("decode opus (%d bytes): %w", len(opusData), err)
	}
	pcm = pcm[:n]

	us.insertSilenceForGap(packet.Timestamp)
	us.lastTS = packet.Timestamp
	us.hasFirstTS = true

	if err := us.wav.Write(pcm); err != nil {
		return err
	}
	if us.liveBuf != nil {
		us.liveBuf.AddSamples(pcm)
	} else {
		log.Printf("Warning: no LiveBuffer for user %s, live transcription disabled", us.userID)
	}
	return nil
}

// decodePLC runs opus Packet Loss Concealment for a missing frame.
// Keeps the decoder state in sync so the next real frame decodes cleanly.
func (us *UserStream) decodePLC(timestamp uint32) {
	if !us.daveActive {
		return
	}
	pcm := make([]int16, frameSamples)
	n, err := us.decoder.Decode(nil, pcm)
	if err != nil || n == 0 {
		return
	}

	us.insertSilenceForGap(timestamp)
	us.lastTS = timestamp
	us.hasFirstTS = true

	// Best-effort write; errors during loss recovery are not fatal.
	us.wav.Write(pcm[:n])
}

// insertSilenceForGap writes silence into the WAV based on the RTP timestamp
// gap between the last packet and the current one. Discord's RTP timestamps
// run at 48kHz and continue incrementing even when no packets are sent, so
// the gap accurately represents how long the user was silent.
func (us *UserStream) insertSilenceForGap(timestamp uint32) {
	if !us.hasFirstTS {
		return
	}
	expected := us.lastTS + uint32(frameSamples)
	if timestamp <= expected {
		return
	}
	gap := int(timestamp - expected)
	silence := make([]int16, gap)
	us.wav.Write(silence)
	if us.liveBuf != nil {
		us.liveBuf.AddSamples(silence)
	}
}

// InsertSilenceDuration writes the given duration of silence into the WAV file
// and live buffer. Used to fill the gap when a user disconnects and reconnects.
func (us *UserStream) InsertSilenceDuration(d time.Duration) {
	samples := int(d.Seconds() * sampleRate)
	if samples <= 0 {
		return
	}
	silence := make([]int16, samples)
	us.wav.Write(silence)
	if us.liveBuf != nil {
		us.liveBuf.AddSamples(silence)
	}
	// Reset RTP timestamp tracking so the next packet doesn't trigger
	// insertSilenceGap with a stale lastTS.
	us.hasFirstTS = false
	log.Printf("Inserted %.1fs silence for user %s (reconnect gap)", d.Seconds(), us.userID)
}

func (us *UserStream) Close() error {
	if us.liveBuf != nil {
		us.liveBuf.Flush()
	}
	return us.wav.Close()
}
func (us *UserStream) FilePath() string { return us.wav.file.Name() }

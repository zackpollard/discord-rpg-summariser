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
// StreamStatus describes the current state of a user's recording stream.
// Exposed through VoiceActivity for display in the live UI.
type StreamStatus string

const (
	StatusHandshaking  StreamStatus = "handshaking"
	StatusActive       StreamStatus = "active"
	StatusDecryptFail  StreamStatus = "decrypt_failed"
	StatusReconnecting StreamStatus = "reconnecting"
)

type UserStream struct {
	userID         string
	wav            *WAVWriter
	decoder        *opus.Decoder
	lastTS         uint32
	hasFirstTS     bool
	firstPacketAt  time.Time // wall clock of first decoded audio packet
	channelJoinAt  time.Time // wall clock when VSU said user was first in the channel
	daveState      *discordgo.ReceiverState
	daveActive     bool                       // true after the first successful DAVE decrypt
	daveFailCount  int                        // consecutive decryption failures
	nonDaveCount   int                        // non-DAVE non-silence frames while DAVE was active
	daveVC         *discordgo.VoiceConnection // for re-deriving keys
	daveEpoch     uint64                     // last observed DAVE epoch; change → rederive
	prevDaveState *discordgo.ReceiverState   // previous-epoch key, retained as fallback for in-flight stale packets
	liveBuf       *LiveBuffer
	status          StreamStatus
	statusMsg       string // optional human-readable detail (e.g. "waiting for exporter secret")
	lostPacketCount int    // packets we couldn't decrypt (pre-handshake / key gap)

	// Drift detection buffer. We capture (RTP_TS, wall_received) pairs for
	// the first N successfully decoded packets, then compute steady-state
	// drift. If Discord buffered the stream during handshake and burst the
	// buffer, drift is negative by the buffer depth and we correct
	// firstPacketAt backward by that amount.
	rtpTraceSamples        int
	rtpTraceRTP            [rtpTraceSize]uint32
	rtpTraceWall           [rtpTraceSize]time.Time
	rtpTraceDumped         bool
	onFirstPacketCorrected func() // called once after drift correction so the recorder can rewrite offsets.json
}

// driftCorrectionThreshold is the minimum absolute drift (seconds) at which
// we apply a firstPacketAt correction. 30ms is above realistic network
// jitter but still catches client-side buffers (e.g. WebRTC sender-side
// jitter buffer, ~100ms) that otherwise slip through.
const driftCorrectionThreshold = 0.030

const rtpTraceSize = 750 // ~15 seconds of RTP — long enough to outlast Discord's accelerated delivery and reach plateau

// decryptFailThreshold is the number of consecutive decryption failures
// after which we consider the stream stuck and report decrypt_failed.
const decryptFailThreshold = 50

// Status returns the stream's current status (handshaking/active/etc) and
// an optional human-readable detail string.
func (us *UserStream) Status() (StreamStatus, string) {
	return us.status, us.statusMsg
}

// LostPackets returns the number of DAVE packets dropped before the
// handshake completed (for monitoring).
func (us *UserStream) LostPackets() int {
	return us.lostPacketCount
}

// FirstPacketAt returns the wall-clock time of the first decoded audio
// packet, or the zero time if no audio has been received yet.
func (us *UserStream) FirstPacketAt() time.Time {
	return us.firstPacketAt
}

// resetDecoder discards opus decoder state. Called on reconnect since the
// new SSRC corresponds to a fresh opus stream and decoding new packets
// against an old state produces audible artifacts.
func (us *UserStream) resetDecoder() error {
	dec, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return err
	}
	us.decoder = dec
	return nil
}

// recordLostPacket tracks a packet we couldn't decrypt (pre-handshake retry
// or post-handshake decrypt failure). firstPacketAt is deliberately NOT
// stamped here — the silence-pad block in HandlePacket is the sole writer so
// offsets.json reflects the true track-start anchor (channel-join time).
func (us *UserStream) recordLostPacket(_ *discordgo.Packet) {
	us.lostPacketCount++
}

// dumpRTPTrace analyses the captured (RTP_TS, wall_received) pairs and, if
// Discord delivered the stream at non-realtime rate (buffer dump or
// sustained accelerated delivery), returns the measured steady-state drift.
//
// A stream that's delivered live has drift ≈ 0 from the start and stays
// there (within network jitter). A stream that's buffered starts at drift=0
// and accumulates negative drift as packets arrive faster than they were
// produced, until the buffer drains and drift plateaus at -buffer_depth.
//
// Detection: walk a sliding window of K packets over the trace and pick
// the earliest index where the drift is plateau'd (max - min within the
// window < tolerance). That's the end of the accelerated-delivery regime.
//
// Returns the drift in wall-clock seconds. Negative drift = Discord
// buffered the stream; add it to firstPacketAt to shift the track earlier.
// Returns 0 if the trace never plateau'd (buffer deeper than trace span).
func (us *UserStream) dumpRTPTrace() float64 {
	us.rtpTraceDumped = true
	base := us.rtpTraceWall[0]
	baseRTP := us.rtpTraceRTP[0]

	drifts := make([]float64, us.rtpTraceSamples)
	for i := 0; i < us.rtpTraceSamples; i++ {
		wallOff := us.rtpTraceWall[i].Sub(base).Seconds()
		rtpOff := float64(int64(us.rtpTraceRTP[i])-int64(baseRTP)) / sampleRate
		drifts[i] = wallOff - rtpOff
	}

	const plateauWindow = 30
	const plateauTol = 0.030
	plateauAt := -1
	var plateauDrift float64
	for i := plateauWindow; i < len(drifts); i++ {
		mn, mx := drifts[i-plateauWindow], drifts[i-plateauWindow]
		for j := i - plateauWindow + 1; j <= i; j++ {
			if drifts[j] < mn {
				mn = drifts[j]
			}
			if drifts[j] > mx {
				mx = drifts[j]
			}
		}
		if mx-mn <= plateauTol {
			plateauAt = i
			// Average across the window for a cleaner estimate.
			var sum float64
			for j := i - plateauWindow + 1; j <= i; j++ {
				sum += drifts[j]
			}
			plateauDrift = sum / float64(plateauWindow)
			break
		}
	}

	last := us.rtpTraceSamples - 1
	log.Printf("RTP_TRACE user=%s samples=%d last_drift=%.4fs min_drift=%.4fs plateau_idx=%d plateau_drift=%.4fs",
		us.userID, us.rtpTraceSamples, drifts[last], minFloat(drifts), plateauAt, plateauDrift)

	return plateauDrift
}

func minFloat(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, v := range xs {
		if v < m {
			m = v
		}
	}
	return m
}

// NewUserStream creates a WAV writer and opus decoder for the given user.
// vc is stored for DAVE key re-derivation after epoch transitions.
func NewUserStream(userID, outputDir string, daveState *discordgo.ReceiverState, vc *discordgo.VoiceConnection) (*UserStream, error) {
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

	us := &UserStream{
		userID:    userID,
		wav:       w,
		decoder:   dec,
		daveState: daveState,
		daveVC:    vc,
		status:    StatusHandshaking,
	}
	if daveState == nil {
		us.statusMsg = "waiting for DAVE exporter secret"
	} else {
		us.statusMsg = "waiting for first audio packet"
	}
	return us, nil
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
// vc is passed to allow re-deriving DAVE keys after epoch transitions.
func (us *UserStream) HandlePacket(packet *discordgo.Packet) error {
	// Proactive epoch check. On epoch change, rederive to get the new
	// receiver key — but keep the previous key as a fallback so in-flight
	// packets from the old epoch still decrypt cleanly (the fork's GCM
	// impl now validates auth tags, so we can tell which key belongs to
	// which frame by trying both).
	if us.daveVC != nil {
		if dave := us.daveVC.DAVESession(); dave != nil {
			if ep := dave.Epoch(); ep != us.daveEpoch {
				if us.daveEpoch != 0 {
					log.Printf("DAVE epoch change for %s: %d -> %d", us.userID, us.daveEpoch, ep)
				}
				us.daveEpoch = ep
				us.rederiveDAVE()
			}
		}
	}

	opusData := packet.Opus

	daveFrame, isDave := findDAVEFrame(opusData)
	if isDave {
		if us.daveState == nil {
			// Key derivation failed earlier (e.g. DAVE session not yet
			// re-handshaked after a reconnect). Track this packet as a lost
			// "handshake gap" and retry on each subsequent packet until the
			// session has an exporter secret available.
			if !us.rederiveDAVE() {
				us.recordLostPacket(packet)
				return nil
			}
		}
		decrypted, err := discordgo.DecryptFrame(us.daveState, daveFrame)
		if err != nil {
			// Current-epoch key failed. If we kept an old-epoch key after a
			// recent rederive, try that — the packet may still be in flight
			// from before the transition.
			if us.prevDaveState != nil {
				if alt, altErr := discordgo.DecryptFrame(us.prevDaveState, daveFrame); altErr == nil {
					decrypted = alt
					err = nil
				}
			}
			// Still failing — maybe the session rotated again since our
			// last rederive. Try rederiving once more.
			if err != nil && us.rederiveDAVE() {
				decrypted, err = discordgo.DecryptFrame(us.daveState, daveFrame)
			}
			if err != nil {
				us.daveFailCount++
				if us.daveFailCount <= 3 || us.daveFailCount%100 == 0 {
					log.Printf("DAVE decrypt failed for %s (seq=%d, %d bytes, failures=%d): %v",
						us.userID, packet.Sequence, len(daveFrame), us.daveFailCount, err)
				}
				if us.daveFailCount >= decryptFailThreshold {
					us.status = StatusDecryptFail
					us.statusMsg = fmt.Sprintf("%d consecutive decrypt failures", us.daveFailCount)
				}
				us.recordLostPacket(packet)
				return nil
			}
		}
		us.daveFailCount = 0
		opusData = decrypted
		us.daveActive = true
	} else if us.daveActive {
		if isOpusSilence(opusData) {
			// Falls through to normal opus decode below.
		} else {
			// Once DAVE is active, non-silence non-DAVE frames shouldn't
			// reach us. An epoch transition may have just shifted framing
			// — rederive and drop the packet. The next good packet's
			// insertSilenceForGap fills the gap based on RTP timestamps.
			us.nonDaveCount++
			if us.nonDaveCount <= 3 || us.nonDaveCount%100 == 0 {
				log.Printf("non-DAVE non-silence frame for %s (seq=%d, %d bytes, count=%d) — rederiving",
					us.userID, packet.Sequence, len(opusData), us.nonDaveCount)
			}
			us.rederiveDAVE()
			return nil
		}
	} else {
		return nil // skip everything before DAVE is active
	}

	pcm := make([]int16, maxPCMFrameSize)
	n, err := us.decoder.Decode(opusData, pcm)
	if err != nil {
		// GCM auth now rejects wrong-key decrypts, so reaching this path
		// means the plaintext is genuine but opus couldn't make sense of
		// the frame (truly corrupt). Drop — the next good packet's
		// insertSilenceForGap will fill the gap from the RTP timestamp.
		us.daveFailCount++
		if us.daveFailCount <= 3 || us.daveFailCount%100 == 0 {
			log.Printf("opus decode failed for %s (seq=%d, %d bytes, failures=%d): %v",
				us.userID, packet.Sequence, len(opusData), us.daveFailCount, err)
		}
		return nil
	}
	pcm = pcm[:n]

	now := time.Now()

	// Capture (RTP_TS, wall_received) for drift detection. Once the trace
	// is full, we analyse the pattern: if the start was a Discord buffer
	// dump (ΔW << ΔRTP at packet 0, stabilising after a few packets), the
	// steady-state drift is negative by the buffer depth. We back-project
	// firstPacketAt by that amount so the track plays at content-time
	// instead of bot-receive-time in the mix.
	if !us.rtpTraceDumped && us.rtpTraceSamples < rtpTraceSize {
		us.rtpTraceRTP[us.rtpTraceSamples] = packet.Timestamp
		us.rtpTraceWall[us.rtpTraceSamples] = now
		us.rtpTraceSamples++
		if us.rtpTraceSamples == rtpTraceSize {
			drift := us.dumpRTPTrace()
			if drift < -driftCorrectionThreshold && !us.firstPacketAt.IsZero() {
				old := us.firstPacketAt
				us.firstPacketAt = us.firstPacketAt.Add(time.Duration(drift * float64(time.Second)))
				log.Printf("BUFFER_CORRECTION user=%s drift=%.4fs firstPacketAt %s -> %s",
					us.userID, drift, old.Format(time.RFC3339Nano), us.firstPacketAt.Format(time.RFC3339Nano))
				if us.onFirstPacketCorrected != nil {
					us.onFirstPacketCorrected()
				}
			}
		}
	}

	if !us.hasFirstTS {
		if us.firstPacketAt.IsZero() {
			// Fresh stream. Under the "Discord skips ahead during DAVE
			// negotiation" model, the first decoded packet's content is
			// from the user's client clock at approximately *now* (not at
			// VSU time) — Discord drops audio during negotiation and
			// resumes live once keys are established. So anchor firstPacketAt
			// to now. No silence pad between VSU and now is needed, because
			// the audio Discord would have delivered during that window was
			// never sent.
			us.firstPacketAt = now
			log.Printf("FIRST_PACKET user=%s ssrc=%d rtp_ts=%d wall=%s (fresh stream, anchored to first-decoded)",
				us.userID, packet.SSRC, packet.Timestamp, us.firstPacketAt.Format(time.RFC3339Nano))
		} else {
			// Reconnect. firstPacketAt is preserved from the original session
			// so offsets.json still points at the true session-start anchor.
			// InsertSilenceDuration has already filled the WAV up to
			// channelJoinAt (the rejoin VSU); pad the remaining gap from
			// rejoin to first new audio.
			anchor := now
			if !us.channelJoinAt.IsZero() && us.channelJoinAt.Before(now) {
				anchor = us.channelJoinAt
			}
			gap := now.Sub(anchor)
			if gap > 50*time.Millisecond {
				samples := int(gap.Seconds() * sampleRate)
				silence := make([]int16, samples)
				us.wav.Write(silence)
				if us.liveBuf != nil {
					us.liveBuf.AddSamples(silence)
				}
				log.Printf("Padded %.2fs silence for %s (rejoin to first-audio gap)",
					gap.Seconds(), us.userID)
			}
			log.Printf("FIRST_PACKET user=%s ssrc=%d rtp_ts=%d wall=%s (reconnect, anchor preserved)",
				us.userID, packet.SSRC, packet.Timestamp, us.firstPacketAt.Format(time.RFC3339Nano))
		}
	}
	us.insertSilenceForGap(packet.Timestamp)
	us.lastTS = packet.Timestamp
	us.hasFirstTS = true
	us.status = StatusActive
	us.statusMsg = ""

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

// rederiveDAVE attempts to re-derive the DAVE receiver key from the current
// voice connection's DAVE session. Called on epoch transitions (e.g. a new
// user joins) and as a retry on decrypt failure. Returns true if successful.
//
// The previous receiver state is retained as us.prevDaveState so that
// in-flight packets encrypted under the old epoch still decrypt cleanly —
// the fork's GCM auth now validates tags, so the caller can try both keys
// and know which frame belongs to which epoch.
func (us *UserStream) rederiveDAVE() bool {
	if us.daveVC == nil {
		return false
	}
	dave := us.daveVC.DAVESession()
	if dave == nil {
		return false
	}
	rs, err := dave.DeriveReceiverKey(us.userID)
	if err != nil {
		return false
	}
	if us.daveState != nil {
		us.prevDaveState = us.daveState
	}
	us.daveState = rs
	log.Printf("Re-derived DAVE receiver key for %s (prev retained as fallback)", us.userID)
	return true
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
		us.liveBuf.Close()
	}
	return us.wav.Close()
}
func (us *UserStream) FilePath() string { return us.wav.file.Name() }


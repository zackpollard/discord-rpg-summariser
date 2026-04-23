package voice

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	maxPendingPackets = 100 // ~2 seconds at 50 packets/sec
	activityTimeout   = 300 * time.Millisecond
)

// UserActivity tracks live voice activity for a single user.
type UserActivity struct {
	UserID        string       `json:"user_id"`
	DisplayName   string       `json:"display_name"`
	Speaking      bool         `json:"speaking"`
	PacketCount   int64        `json:"packet_count"`
	LastPacketAt  time.Time    `json:"last_packet_at"`
	Status        StreamStatus `json:"status"`         // handshaking/active/decrypt_failed/reconnecting
	StatusMessage string       `json:"status_message"` // optional human-readable detail
	LostPackets   int          `json:"lost_packets"`   // DAVE packets lost before handshake completed
}

// NameResolver resolves a Discord user ID to a display name.
type NameResolver func(userID string) string

// Recorder manages per-user recording for a voice session. It maps SSRCs to
// users, decrypts DAVE frames, decodes opus, and writes per-user WAV files.
type Recorder struct {
	mu             sync.Mutex
	streams        map[uint32]*UserStream
	ssrcToUser     map[uint32]string
	userToSSRC     map[string]uint32
	pendingPackets map[uint32][]*discordgo.Packet
	activity       map[string]*UserActivity
	userJoinOffset map[string]time.Duration // userID -> offset from session start to first audio
	vsuJoinAt      map[string]time.Time     // userID -> wall-clock of VoiceStateUpdate channel join
	outputDir      string
	guildID        string
	done           chan struct{}
	liveCh         chan ChunkReady // nil if live transcription disabled
	sessionStart   time.Time
}

func NewRecorder(outputDir, guildID string, liveCh chan ChunkReady) *Recorder {
	return &Recorder{
		streams:        make(map[uint32]*UserStream),
		ssrcToUser:     make(map[uint32]string),
		userToSSRC:     make(map[string]uint32),
		pendingPackets: make(map[uint32][]*discordgo.Packet),
		activity:       make(map[string]*UserActivity),
		userJoinOffset: make(map[string]time.Duration),
		vsuJoinAt:      make(map[string]time.Time),
		outputDir:      outputDir,
		guildID:        guildID,
		done:           make(chan struct{}),
		liveCh:         liveCh,
		sessionStart:   time.Now(),
	}
}

// RecordVoiceStateJoin records the wall-clock time at which a user entered
// the recorded voice channel (via VoiceStateUpdate or initial enumeration).
// Overwrites any previous timestamp — for rejoins within the same session we
// need the most recent join time, not the original one. Paired with
// RecordVoiceStateLeave which clears the field on channel-leave.
//
// If a stream already exists for this user (rare VSU-after-speaking-update
// ordering) channelJoinAt is updated so the next silence-pad uses the fresh
// value.
func (r *Recorder) RecordVoiceStateJoin(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.vsuJoinAt[userID] = now
	for _, us := range r.streams {
		if us.userID == userID {
			us.channelJoinAt = now
			break
		}
	}
}

// RecordVoiceStateLeave clears the recorded VSU join time so that the next
// join VSU is captured fresh rather than being ignored. Called when a user
// moves out of the recorded channel or disconnects.
func (r *Recorder) RecordVoiceStateLeave(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.vsuJoinAt, userID)
}

// HandleSpeakingUpdate maps an SSRC to a user ID, creates a UserStream if
// needed, derives the DAVE receiver key, and flushes any buffered packets.
// If a user reconnects with a new SSRC, the existing stream is reused and
// remapped so audio continues in the same WAV file.
func (r *Recorder) HandleSpeakingUpdate(vc *discordgo.VoiceConnection, ssrc uint32, userID, displayName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ssrcToUser[ssrc] = userID

	if _, ok := r.activity[userID]; !ok {
		r.activity[userID] = &UserActivity{UserID: userID, DisplayName: displayName}
	} else if displayName != "" {
		r.activity[userID].DisplayName = displayName
	}

	// Stream already exists for this exact SSRC — nothing to do.
	if _, ok := r.streams[ssrc]; ok {
		return
	}

	// Derive DAVE receiver key for this user.
	var daveState *discordgo.ReceiverState
	if dave := vc.DAVESession(); dave != nil {
		rs, err := dave.DeriveReceiverKey(userID)
		if err != nil {
			log.Printf("Warning: DAVE key derivation failed for %s: %v", userID, err)
		} else {
			daveState = rs
		}
	}

	// Check if this user already has a stream under a previous SSRC (reconnect).
	if oldSSRC, ok := r.userToSSRC[userID]; ok && oldSSRC != ssrc {
		if existing, ok := r.streams[oldSSRC]; ok {
			// Anchor the rejoin to the VSU time if we have one (cleared on
			// leave, set on rejoin). Fall back to now if VSU hasn't arrived
			// yet (rare: speaking-update before VSU).
			rejoinAnchor := time.Now()
			if vsu, ok := r.vsuJoinAt[userID]; ok && !vsu.IsZero() {
				rejoinAnchor = vsu
			}
			// Insert silence to fill the WAV from the last real packet up
			// to the rejoin anchor. The silence-pad block in HandlePacket
			// will cover the remaining rejoin-to-first-audio gap.
			if a, ok := r.activity[userID]; ok && !a.LastPacketAt.IsZero() {
				if gap := rejoinAnchor.Sub(a.LastPacketAt); gap > 0 {
					existing.InsertSilenceDuration(gap)
				} else {
					// Clock/ordering anomaly — just reset hasFirstTS and rely
					// on the silence-pad block.
					existing.hasFirstTS = false
				}
			} else {
				existing.hasFirstTS = false
			}

			// Reuse the existing stream under the new SSRC.
			existing.daveState = daveState
			existing.daveVC = vc
			existing.daveActive = false // reset so new DAVE handshake can proceed
			existing.status = StatusReconnecting
			existing.channelJoinAt = rejoinAnchor
			if daveState == nil {
				existing.statusMsg = "reconnected — waiting for DAVE handshake"
			} else {
				existing.statusMsg = "reconnected — waiting for first audio packet"
			}
			// Reset the opus decoder — the new SSRC starts a fresh opus
			// stream, and reusing the old decoder's state produces artifacts.
			if err := existing.resetDecoder(); err != nil {
				log.Printf("failed to reset opus decoder for %s: %v", userID, err)
			}
			r.streams[ssrc] = existing
			delete(r.streams, oldSSRC)
			r.userToSSRC[userID] = ssrc
			log.Printf("User %s (%s) reconnected with new SSRC %d (was %d)", displayName, userID, ssrc, oldSSRC)

			if pending, ok := r.pendingPackets[ssrc]; ok {
				for _, pkt := range pending {
					existing.HandlePacket(pkt)
				}
				r.activity[userID].PacketCount += int64(len(pending))
				delete(r.pendingPackets, ssrc)
			}
			return
		}
	}

	r.userToSSRC[userID] = ssrc

	us, err := NewUserStream(userID, r.outputDir, daveState, vc)
	if err != nil {
		log.Printf("Failed to create stream for user %s: %v", userID, err)
		return
	}
	// VoiceStateUpdate fires before SpeakingUpdate for new joiners, so the
	// VSU time is already recorded in vsuJoinAt. Propagate it to the stream
	// so it can pad silence for the VSU-to-first-audio gap.
	if vsuTime, ok := r.vsuJoinAt[userID]; ok {
		us.channelJoinAt = vsuTime
	}
	// Rewrite offsets.json when the stream detects Discord jitter-buffer
	// drift and back-projects its firstPacketAt. The callback runs under
	// r.mu because HandlePacket is called with the lock held.
	us.onFirstPacketCorrected = func() {
		r.writeOffsetsLocked()
	}
	joinOffset := time.Since(r.sessionStart)
	r.userJoinOffset[userID] = joinOffset
	if r.liveCh != nil {
		us.liveBuf = NewLiveBuffer(userID, displayName, r.sessionStart, joinOffset, r.liveCh)
	}
	r.streams[ssrc] = us
	log.Printf("Recording user %s (%s) join_offset=%.1fs", displayName, userID, joinOffset.Seconds())

	// Persist offsets to disk immediately so they survive crashes.
	r.writeOffsetsLocked()

	if pending, ok := r.pendingPackets[ssrc]; ok {
		for _, pkt := range pending {
			us.HandlePacket(pkt) // errors expected for pre-DAVE packets
		}
		r.activity[userID].PacketCount += int64(len(pending))
		delete(r.pendingPackets, ssrc)
		// Pending packet drain may have set firstPacketAt via the silence-pad
		// path. Re-persist offsets.json so it reflects the tighter timing.
		if !us.FirstPacketAt().IsZero() {
			r.writeOffsetsLocked()
		}
	}
}

// writeOffsetsLocked writes the current join offsets to offsets.json in the
// output directory. Caller must hold r.mu.
//
// The offset is the time from session start to the user's first decoded
// audio packet, measured in the audio receive goroutine (tight, unaffected
// by CPU starvation of event handlers). Falls back to the speaking-update
// stamped value when the user has no decoded audio yet.
func (r *Recorder) writeOffsetsLocked() {
	offsets := make(map[string]float64, len(r.userJoinOffset))
	for userID, fallback := range r.userJoinOffset {
		offsets[userID] = fallback.Seconds()
	}
	for _, us := range r.streams {
		if fpa := us.FirstPacketAt(); !fpa.IsZero() {
			offsets[us.userID] = fpa.Sub(r.sessionStart).Seconds()
		}
	}
	data, err := json.Marshal(offsets)
	if err != nil {
		return
	}
	path := filepath.Join(r.outputDir, "offsets.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Printf("Failed to write %s: %v", path, err)
	}
}

// HandleVoicePacket routes a packet to the correct UserStream by SSRC.
// Unknown SSRCs are buffered until a speaking update maps them.
func (r *Recorder) HandleVoicePacket(packet *discordgo.Packet) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if us, ok := r.streams[packet.SSRC]; ok {
		hadFirstPacket := !us.FirstPacketAt().IsZero()
		if err := us.HandlePacket(packet); err != nil {
			uid := r.ssrcToUser[packet.SSRC]
			log.Printf("Error handling packet for %s: %v", uid, err)
		}
		uid := r.ssrcToUser[packet.SSRC]
		if a, ok := r.activity[uid]; ok {
			a.PacketCount++
			a.LastPacketAt = time.Now()
		}
		// Re-persist offsets with the tighter first-packet wall-clock time
		// the very first time we decode audio from this user (not on reconnect).
		if !hadFirstPacket && !us.FirstPacketAt().IsZero() {
			r.writeOffsetsLocked()
		}
		return
	}

	buf := r.pendingPackets[packet.SSRC]
	if len(buf) >= maxPendingPackets {
		buf = buf[1:]
	}
	r.pendingPackets[packet.SSRC] = append(buf, packet)
}

// Start registers handlers and begins reading voice packets.
func (r *Recorder) Start(vc *discordgo.VoiceConnection, nameResolver NameResolver) {
	log.Printf("Recorder starting (OpusRecv=%v)", vc.OpusRecv != nil)

	vc.AddHandler(func(vc *discordgo.VoiceConnection, vs *discordgo.VoiceSpeakingUpdate) {
		log.Printf("Speaking update: user=%s ssrc=%d", vs.UserID, vs.SSRC)
		name := ""
		if nameResolver != nil {
			name = nameResolver(vs.UserID)
		}
		r.HandleSpeakingUpdate(vc, uint32(vs.SSRC), vs.UserID, name)
	})

	go func() {
		var pktCount int64
		for {
			select {
			case <-r.done:
				log.Printf("Recorder stopped after %d packets", pktCount)
				return
			case pkt, ok := <-vc.OpusRecv:
				if !ok {
					log.Printf("OpusRecv channel closed after %d packets", pktCount)
					return
				}
				if pkt != nil {
					pktCount++
					if pktCount == 1 {
						log.Printf("First voice packet received (SSRC=%d)", pkt.SSRC)
					}
					r.HandleVoicePacket(pkt)
				}
			}
		}
	}()
}

// Stop signals the receive goroutine to exit and closes all streams.
func (r *Recorder) Stop() error {
	close(r.done)

	r.mu.Lock()
	defer r.mu.Unlock()

	var firstErr error
	for ssrc, us := range r.streams {
		if err := us.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close stream for SSRC %d: %w", ssrc, err)
		}
	}
	r.writeTimingDebugLocked()
	if r.liveCh != nil {
		close(r.liveCh)
	}
	return firstErr
}

// writeTimingDebugLocked persists all per-user timing signals to
// timing-debug.json for post-hoc drift investigation. Includes the
// speaking-update offset, the first-packet-receive offset, and the
// voice-state-update channel-join offset.
func (r *Recorder) writeTimingDebugLocked() {
	type userTiming struct {
		SpeakingUpdateOffset float64 `json:"speaking_update_offset_sec"`
		FirstPacketOffset    float64 `json:"first_packet_offset_sec"`
		VoiceStateOffset     float64 `json:"voice_state_offset_sec"`
		HasFirstPacket       bool    `json:"has_first_packet"`
		HasVoiceStateJoin    bool    `json:"has_voice_state_join"`
	}

	users := make(map[string]userTiming)
	add := func(uid string) {
		if _, ok := users[uid]; ok {
			return
		}
		users[uid] = userTiming{}
	}
	for uid := range r.userJoinOffset {
		add(uid)
	}
	for uid := range r.vsuJoinAt {
		add(uid)
	}
	for _, us := range r.streams {
		add(us.userID)
	}

	for uid := range users {
		t := users[uid]
		if d, ok := r.userJoinOffset[uid]; ok {
			t.SpeakingUpdateOffset = d.Seconds()
		}
		if v, ok := r.vsuJoinAt[uid]; ok {
			t.VoiceStateOffset = v.Sub(r.sessionStart).Seconds()
			t.HasVoiceStateJoin = true
		}
		for _, us := range r.streams {
			if us.userID == uid {
				if fpa := us.FirstPacketAt(); !fpa.IsZero() {
					t.FirstPacketOffset = fpa.Sub(r.sessionStart).Seconds()
					t.HasFirstPacket = true
				}
				break
			}
		}
		users[uid] = t
	}

	payload := struct {
		SessionStart string                `json:"session_start"`
		Users        map[string]userTiming `json:"users"`
	}{
		SessionStart: r.sessionStart.Format(time.RFC3339Nano),
		Users:        users,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(r.outputDir, "timing-debug.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Printf("Failed to write %s: %v", path, err)
	}
}

// UserFiles returns userID → WAV file path for every recorded user.
func (r *Recorder) UserFiles() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	files := make(map[string]string, len(r.streams))
	for ssrc, us := range r.streams {
		files[r.ssrcToUser[ssrc]] = us.FilePath()
	}
	return files
}

// UserJoinOffsets returns userID → duration from session start to first audio
// for each recorded user. Used to adjust transcript timestamps during merge.
//
// Prefers the first-packet wall-clock time stamped in the audio receive
// goroutine (tight, unaffected by event-handler CPU starvation). Falls back
// to the speaking-update stamped value when no audio was decoded.
func (r *Recorder) UserJoinOffsets() map[string]time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	offsets := make(map[string]time.Duration, len(r.userJoinOffset))
	for k, v := range r.userJoinOffset {
		offsets[k] = v
	}
	for _, us := range r.streams {
		if fpa := us.FirstPacketAt(); !fpa.IsZero() {
			offsets[us.userID] = fpa.Sub(r.sessionStart)
		}
	}
	return offsets
}

// Activity returns a snapshot of per-user voice activity.
func (r *Recorder) Activity() []UserActivity {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Build a quick userID -> stream lookup for status/pending info.
	streamByUser := make(map[string]*UserStream, len(r.streams))
	for _, us := range r.streams {
		streamByUser[us.userID] = us
	}

	now := time.Now()
	out := make([]UserActivity, 0, len(r.activity))
	for _, a := range r.activity {
		snap := *a
		snap.Speaking = now.Sub(a.LastPacketAt) < activityTimeout
		if us, ok := streamByUser[a.UserID]; ok {
			snap.Status, snap.StatusMessage = us.Status()
			snap.LostPackets = us.LostPackets()
		}
		out = append(out, snap)
	}
	return out
}

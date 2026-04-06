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
	UserID       string    `json:"user_id"`
	DisplayName  string    `json:"display_name"`
	Speaking     bool      `json:"speaking"`
	PacketCount  int64     `json:"packet_count"`
	LastPacketAt time.Time `json:"last_packet_at"`
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
		outputDir:      outputDir,
		guildID:        guildID,
		done:           make(chan struct{}),
		liveCh:         liveCh,
		sessionStart:   time.Now(),
	}
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
			// Insert silence for the duration the user was disconnected.
			if a, ok := r.activity[userID]; ok && !a.LastPacketAt.IsZero() {
				gap := time.Since(a.LastPacketAt)
				existing.InsertSilenceDuration(gap)
			}

			// Reuse the existing stream under the new SSRC.
			existing.daveState = daveState
			existing.daveVC = vc
			existing.daveActive = false // reset so new DAVE handshake can proceed
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
	}
}

// writeOffsetsLocked writes the current join offsets to offsets.json in the
// output directory. Caller must hold r.mu.
func (r *Recorder) writeOffsetsLocked() {
	offsets := make(map[string]float64, len(r.userJoinOffset))
	for userID, d := range r.userJoinOffset {
		offsets[userID] = d.Seconds()
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
		if err := us.HandlePacket(packet); err != nil {
			uid := r.ssrcToUser[packet.SSRC]
			log.Printf("Error handling packet for %s: %v", uid, err)
		}
		uid := r.ssrcToUser[packet.SSRC]
		if a, ok := r.activity[uid]; ok {
			a.PacketCount++
			a.LastPacketAt = time.Now()
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
	if r.liveCh != nil {
		close(r.liveCh)
	}
	return firstErr
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
func (r *Recorder) UserJoinOffsets() map[string]time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	offsets := make(map[string]time.Duration, len(r.userJoinOffset))
	for k, v := range r.userJoinOffset {
		offsets[k] = v
	}
	return offsets
}

// Activity returns a snapshot of per-user voice activity.
func (r *Recorder) Activity() []UserActivity {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	out := make([]UserActivity, 0, len(r.activity))
	for _, a := range r.activity {
		snap := *a
		snap.Speaking = now.Sub(a.LastPacketAt) < activityTimeout
		out = append(out, snap)
	}
	return out
}

package voice

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	// maxPendingPackets is roughly 2 seconds of audio at 50 packets/sec.
	maxPendingPackets = 100

	// activityTimeout is how long after the last packet a user is still
	// considered "speaking".
	activityTimeout = 300 * time.Millisecond
)

// UserActivity tracks live voice activity for a single user.
type UserActivity struct {
	UserID       string    `json:"user_id"`
	Speaking     bool      `json:"speaking"`
	PacketCount  int64     `json:"packet_count"`
	LastPacketAt time.Time `json:"last_packet_at"`
}

// Recorder manages per-user recording for a voice session.
type Recorder struct {
	mu             sync.Mutex
	streams        map[uint32]*UserStream
	ssrcToUser     map[uint32]string
	userToSSRC     map[string]uint32
	pendingPackets map[uint32][]*discordgo.Packet
	outputDir      string
	guildID        string
	done           chan struct{}

	// activity tracks per-user packet counts and last-seen times.
	activity map[string]*UserActivity
}

// NewRecorder creates a Recorder that writes per-user WAV files into outputDir.
func NewRecorder(outputDir, guildID string) *Recorder {
	return &Recorder{
		streams:        make(map[uint32]*UserStream),
		ssrcToUser:     make(map[uint32]string),
		userToSSRC:     make(map[string]uint32),
		pendingPackets: make(map[uint32][]*discordgo.Packet),
		activity:       make(map[string]*UserActivity),
		outputDir:      outputDir,
		guildID:        guildID,
		done:           make(chan struct{}),
	}
}

// HandleSpeakingUpdate maps an SSRC to a user ID, creates a UserStream if
// needed, and flushes any buffered packets for that SSRC.
func (r *Recorder) HandleSpeakingUpdate(vc *discordgo.VoiceConnection, ssrc uint32, userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("Speaking update: SSRC=%d user=%s", ssrc, userID)

	r.ssrcToUser[ssrc] = userID
	r.userToSSRC[userID] = ssrc

	if _, ok := r.activity[userID]; !ok {
		r.activity[userID] = &UserActivity{UserID: userID}
	}

	if _, ok := r.streams[ssrc]; ok {
		return
	}

	us, err := NewUserStream(userID, r.outputDir)
	if err != nil {
		log.Printf("Failed to create stream for user %s: %v", userID, err)
		return
	}
	r.streams[ssrc] = us
	log.Printf("Created audio stream for user %s", userID)

	// Flush any pending packets that arrived before the speaking update.
	if pending, ok := r.pendingPackets[ssrc]; ok {
		for _, pkt := range pending {
			if err := us.HandlePacket(pkt); err != nil {
				log.Printf("Error flushing pending packet for user %s: %v", userID, err)
			}
		}
		r.activity[userID].PacketCount += int64(len(pending))
		delete(r.pendingPackets, ssrc)
	}
}

// HandleVoicePacket routes a received packet to the correct UserStream by SSRC.
// If the SSRC is not yet known, the packet is buffered up to a 2-second limit.
func (r *Recorder) HandleVoicePacket(packet *discordgo.Packet) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if us, ok := r.streams[packet.SSRC]; ok {
		if err := us.HandlePacket(packet); err != nil {
			uid := r.ssrcToUser[packet.SSRC]
			log.Printf("Error handling packet for user %s: %v", uid, err)
		}
		uid := r.ssrcToUser[packet.SSRC]
		if a, ok := r.activity[uid]; ok {
			a.PacketCount++
			a.LastPacketAt = time.Now()
		}
		return
	}

	// SSRC not yet mapped; buffer the packet.
	buf := r.pendingPackets[packet.SSRC]
	if len(buf) >= maxPendingPackets {
		buf = buf[1:]
	}
	r.pendingPackets[packet.SSRC] = append(buf, packet)
}

// Start registers the speaking handler on the voice connection and launches a
// goroutine that reads from vc.OpusRecv until Stop is called.
func (r *Recorder) Start(vc *discordgo.VoiceConnection) {
	vc.AddHandler(func(vc *discordgo.VoiceConnection, vs *discordgo.VoiceSpeakingUpdate) {
		r.HandleSpeakingUpdate(vc, uint32(vs.SSRC), vs.UserID)
	})

	log.Printf("Recorder started, waiting for OpusRecv packets...")

	go func() {
		count := 0
		for {
			select {
			case <-r.done:
				log.Printf("Recorder stopped after %d total packets", count)
				return
			case pkt, ok := <-vc.OpusRecv:
				if !ok {
					log.Printf("OpusRecv channel closed after %d packets", count)
					return
				}
				if pkt == nil {
					continue
				}
				count++
				if count == 1 {
					log.Printf("First voice packet received (SSRC=%d, %d opus bytes)", pkt.SSRC, len(pkt.Opus))
				}
				r.HandleVoicePacket(pkt)
			}
		}
	}()
}

// Stop signals the receive goroutine to exit and closes all active streams.
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
	return firstErr
}

// UserFiles returns a mapping of user ID to WAV file path for every recorded user.
func (r *Recorder) UserFiles() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	files := make(map[string]string, len(r.streams))
	for ssrc, us := range r.streams {
		uid := r.ssrcToUser[ssrc]
		files[uid] = us.FilePath()
	}
	return files
}

// Activity returns a snapshot of all tracked users and their current speaking
// state. A user is considered speaking if a packet arrived within activityTimeout.
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

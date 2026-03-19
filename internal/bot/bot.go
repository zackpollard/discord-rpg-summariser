package bot

import (
	"context"
	"log"
	"sync"

	"discord-rpg-summariser/internal/config"
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/transcribe"
	"discord-rpg-summariser/internal/voice"

	"github.com/bwmarrin/discordgo"
)

// Bot manages the Discord session and coordinates recording, transcription,
// and summarisation of RPG sessions.
type Bot struct {
	session     *discordgo.Session
	store       *storage.Store
	config      *config.Config
	recorder    *voice.Recorder
	transcriber *transcribe.Transcriber
	summariser  summarise.Summariser
	activeVC        *discordgo.VoiceConnection
	activeChannelID string // voice channel the bot is currently in
	mu              sync.Mutex
	webBaseURL      string

	// registeredCmds holds the IDs of registered slash commands so they can
	// be removed on shutdown.
	registeredCmds []*discordgo.ApplicationCommand

	// sessionID is the DB ID for the currently active recording session.
	sessionID  int64
	liveWorker *voice.LiveWorker
}

// LiveTranscriptWorker returns the current live transcription worker, or nil.
func (b *Bot) LiveTranscriptWorker() *voice.LiveWorker {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.liveWorker
}

// MemberInfo represents a Discord guild member.
type MemberInfo struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"` // nick > global name > username
}

// GuildMembers returns all non-bot members of the configured guild.
func (b *Bot) GuildMembers() []MemberInfo {
	guildID := b.config.Discord.GuildID

	// Fetch from API (state cache only has members seen in events)
	var all []*discordgo.Member
	after := ""
	for {
		batch, err := b.session.GuildMembers(guildID, after, 1000)
		if err != nil || len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		after = batch[len(batch)-1].User.ID
		if len(batch) < 1000 {
			break
		}
	}

	var members []MemberInfo
	for _, m := range all {
		if m.User == nil || m.User.Bot {
			continue
		}
		display := m.User.Username
		if m.User.GlobalName != "" {
			display = m.User.GlobalName
		}
		if m.Nick != "" {
			display = m.Nick
		}
		members = append(members, MemberInfo{
			UserID:      m.User.ID,
			Username:    m.User.Username,
			DisplayName: display,
		})
	}
	return members
}

// ResolveUsername returns a display name for a Discord user ID.
func (b *Bot) ResolveUsername(userID string) string {
	guildID := b.config.Discord.GuildID
	member, err := b.session.GuildMember(guildID, userID)
	if err != nil || member.User == nil {
		return userID
	}
	if member.Nick != "" {
		return member.Nick
	}
	if member.User.GlobalName != "" {
		return member.User.GlobalName
	}
	return member.User.Username
}

// VoiceActivity returns current voice activity. Nil if not recording.
func (b *Bot) VoiceActivity() []voice.UserActivity {
	b.mu.Lock()
	rec := b.recorder
	b.mu.Unlock()
	if rec == nil {
		return nil
	}
	return rec.Activity()
}

// NewBot creates a new Bot with the given dependencies. The Discord session is
// created but not yet opened; call Start to connect.
func NewBot(cfg *config.Config, store *storage.Store, transcriber *transcribe.Transcriber, summariser summarise.Summariser) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsGuildMessages

	b := &Bot{
		session:     dg,
		store:       store,
		config:      cfg,
		transcriber: transcriber,
		summariser:  summariser,
		webBaseURL:  "http://localhost" + cfg.Web.ListenAddr,
	}

	return b, nil
}

// Start opens the Discord connection, registers slash commands, and installs
// event handlers.
func (b *Bot) Start() error {
	// Clean up sessions left in non-terminal states from a previous run.
	ctx := context.Background()
	if n, err := b.store.CleanupStaleSessions(ctx); err != nil {
		log.Printf("Warning: failed to clean up stale sessions: %v", err)
	} else if n > 0 {
		log.Printf("Cleaned up %d stale session(s) from previous run", n)
	}

	b.session.AddHandler(b.handleInteraction)
	b.session.AddHandler(b.handleVoiceStateUpdate)

	if err := b.session.Open(); err != nil {
		return err
	}

	if err := b.RegisterCommands(); err != nil {
		return err
	}

	log.Println("Bot is running.")
	return nil
}

// Stop cleans up: if a recording is active it is stopped, registered slash
// commands are removed, and the Discord session is closed.
func (b *Bot) Stop() error {
	b.mu.Lock()
	if b.recorder != nil {
		if err := b.recorder.Stop(); err != nil {
			log.Printf("Error stopping recorder during shutdown: %v", err)
		}
		b.recorder = nil
	}
	if b.activeVC != nil {
		if err := b.activeVC.Disconnect(context.Background()); err != nil {
			log.Printf("Error disconnecting voice during shutdown: %v", err)
		}
		b.activeVC = nil
	}
	b.mu.Unlock()

	for _, cmd := range b.registeredCmds {
		if err := b.session.ApplicationCommandDelete(b.session.State.User.ID, b.config.Discord.GuildID, cmd.ID); err != nil {
			log.Printf("Error removing command %s: %v", cmd.Name, err)
		}
	}

	return b.session.Close()
}

// RegisterCommands registers all slash commands defined in commands.go with the
// configured guild.
func (b *Bot) RegisterCommands() error {
	b.registeredCmds = make([]*discordgo.ApplicationCommand, 0, len(commands))
	for _, cmd := range commands {
		registered, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, b.config.Discord.GuildID, cmd)
		if err != nil {
			return err
		}
		b.registeredCmds = append(b.registeredCmds, registered)
	}
	return nil
}

// handleVoiceStateUpdate detects when the voice channel the bot is in becomes
// empty (no non-bot users remain) and automatically stops the session.
func (b *Bot) handleVoiceStateUpdate(s *discordgo.Session, vsu *discordgo.VoiceStateUpdate) {
	b.mu.Lock()
	vc := b.activeVC
	channelID := b.activeChannelID
	b.mu.Unlock()

	if vc == nil {
		return
	}

	// Only care about events in the guild we are recording in.
	if vsu.GuildID != vc.GuildID {
		return
	}

	// A user left our channel (their old channel was ours and they either
	// disconnected or moved elsewhere).
	if vsu.BeforeUpdate == nil || vsu.BeforeUpdate.ChannelID != channelID {
		return
	}
	// They stayed in the same channel, nothing changed for us.
	if vsu.ChannelID == channelID {
		return
	}

	// Check if any non-bot users remain.
	guild, err := s.State.Guild(vc.GuildID)
	if err != nil {
		log.Printf("Failed to fetch guild state: %v", err)
		return
	}

	botUserID := s.State.User.ID
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID != channelID {
			continue
		}
		if vs.UserID == botUserID {
			continue
		}
		// At least one non-bot user is still present.
		return
	}

	log.Println("Voice channel emptied, auto-stopping session.")
	b.mu.Lock()
	sessionID := b.sessionID
	b.mu.Unlock()

	userFiles := b.stopRecording()

	if sessionID != 0 {
		ctx := context.Background()
		if err := b.store.EndSession(ctx, sessionID); err != nil {
			log.Printf("EndSession error (auto-stop): %v", err)
		}
		go b.runPipeline(sessionID, userFiles)
	}
}

// stopRecording stops the recorder and disconnects from voice, returning
// the user-to-WAV file mapping before clearing state. Caller must NOT hold b.mu.
func (b *Bot) stopRecording() map[string]string {
	b.mu.Lock()
	defer b.mu.Unlock()

	var userFiles map[string]string

	if b.recorder != nil {
		if err := b.recorder.Stop(); err != nil {
			log.Printf("Error stopping recorder: %v", err)
		}
		userFiles = b.recorder.UserFiles()
		b.recorder = nil
	}
	if b.activeVC != nil {
		if err := b.activeVC.Disconnect(context.Background()); err != nil {
			log.Printf("Error disconnecting from voice: %v", err)
		}
		b.activeVC = nil
	}
	b.activeChannelID = ""
	b.sessionID = 0
	b.liveWorker = nil

	return userFiles
}

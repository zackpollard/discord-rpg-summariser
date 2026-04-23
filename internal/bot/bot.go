package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"discord-rpg-summariser/internal/config"
	"discord-rpg-summariser/internal/diarize"
	"discord-rpg-summariser/internal/embed"
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/telegram"
	"discord-rpg-summariser/internal/transcribe"
	"discord-rpg-summariser/internal/voice"

	"github.com/bwmarrin/discordgo"
)

// TranscriberFactory creates a new Transcriber on demand.
type TranscriberFactory func() (transcribe.Transcriber, error)

// Bot manages the Discord session and coordinates recording, transcription,
// and summarisation of RPG sessions.
type Bot struct {
	session             *discordgo.Session
	store               *storage.Store
	config              *config.Config
	recorder            *voice.Recorder
	transcriberFactory  TranscriberFactory
	transcriber         transcribe.Transcriber // lazy-loaded, nil when idle
	transcriberRefCount int                    // number of active users (live + pipeline)
	summariser          summarise.Summariser
	activeVC            *discordgo.VoiceConnection
	activeChannelID     string // voice channel the bot is currently in
	mu                  sync.Mutex
	webBaseURL          string

	// registeredCmds holds the IDs of registered slash commands so they can
	// be removed on shutdown.
	registeredCmds []*discordgo.ApplicationCommand

	// sessionID is the DB ID for the currently active recording session.
	sessionID              int64
	liveWorker             *voice.LiveWorker
	incrementalTranscriber *voice.IncrementalTranscriber

	// Telegram integration (nil if not configured).
	telegramClient   *telegram.Client
	telegramListener *telegram.Listener

	// Speaker diarization (lazy-initialized on first shared-mic session).
	diarizer     *diarize.Diarizer
	diarizerOnce sync.Once

	// Embedding generation for RAG (nil if not configured).
	embedder embed.Embedder

	// Pipeline progress tracking (non-nil while a pipeline is running).
	progress *PipelineProgress

	// TTS synthesizer for voice-cloned recap playback (nil if not configured).
	ttsSynth interface {
		Synthesize(ctx context.Context, text string, refAudio []float32, refSampleRate int, refText string) ([]float32, int, error)
		SetProgressCallback(fn func(float64))
	}
}

// AcquireTranscriber is the exported version of acquireTranscriber for use
// by the API server's TranscriberProvider interface.
func (b *Bot) AcquireTranscriber() (transcribe.Transcriber, error) {
	return b.acquireTranscriber()
}

// ReleaseTranscriber is the exported version of releaseTranscriber.
func (b *Bot) ReleaseTranscriber() {
	b.releaseTranscriber()
}

// acquireTranscriber loads the transcription model if not already loaded and
// increments the reference count. Callers must call releaseTranscriber when done.
func (b *Bot) acquireTranscriber() (transcribe.Transcriber, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.transcriber == nil {
		log.Println("Loading transcription model...")
		t, err := b.transcriberFactory()
		if err != nil {
			return nil, err
		}
		b.transcriber = t
		log.Println("Transcription model loaded")
	}
	b.transcriberRefCount++
	return b.transcriber, nil
}

// releaseTranscriber decrements the reference count and unloads the model
// when no one is using it, freeing the ONNX Runtime memory arena.
func (b *Bot) releaseTranscriber() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.transcriberRefCount--
	if b.transcriberRefCount <= 0 {
		b.transcriberRefCount = 0
		if b.transcriber != nil {
			log.Println("Unloading transcription model (no active users)")
			b.transcriber.Close()
			b.transcriber = nil
		}
	}
}

// LiveTranscriptWorker returns the current live transcription worker, or nil.
func (b *Bot) LiveTranscriptWorker() *voice.LiveWorker {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.liveWorker
}

// PipelineProgressFor returns the progress tracker if one is active for the
// given session, or nil otherwise.
func (b *Bot) PipelineProgressFor(sessionID int64) *PipelineProgress {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.progress != nil && b.progress.SessionID() == sessionID {
		return b.progress
	}
	return nil
}

// MemberInfo represents a Discord guild member.
type MemberInfo struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"` // nick > global name > username
}

// SyncGuildMembers fetches all guild members from Discord and caches them in the database.
func (b *Bot) SyncGuildMembers() {
	guildID := b.config.Discord.GuildID
	ctx := context.Background()

	var all []*discordgo.Member
	after := ""
	for {
		batch, err := b.session.GuildMembers(guildID, after, 1000)
		if err != nil {
			log.Printf("Failed to fetch guild members (after=%q): %v", after, err)
			break
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		after = batch[len(batch)-1].User.ID
		if len(batch) < 1000 {
			break
		}
	}

	if len(all) == 0 {
		log.Printf("Warning: fetched 0 guild members — check that Server Members Intent is enabled in the Discord Developer Portal")
	}

	var users []storage.DiscordUser
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
		users = append(users, storage.DiscordUser{
			UserID:      m.User.ID,
			GuildID:     guildID,
			Username:    m.User.Username,
			DisplayName: display,
		})
	}

	if err := b.store.UpsertDiscordUsers(ctx, users); err != nil {
		log.Printf("Failed to sync guild members: %v", err)
	} else {
		log.Printf("Synced %d guild members to database", len(users))
	}
}

// GuildMembers returns cached guild members from the database.
func (b *Bot) GuildMembers() []MemberInfo {
	users, err := b.store.GetDiscordUsers(context.Background(), b.config.Discord.GuildID)
	if err != nil {
		log.Printf("Failed to get guild members from DB: %v", err)
		return nil
	}
	members := make([]MemberInfo, len(users))
	for i, u := range users {
		members[i] = MemberInfo{
			UserID:      u.UserID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
		}
	}
	return members
}

// ResolveUsername returns a display name for a Discord user ID from the DB cache.
func (b *Bot) ResolveUsername(userID string) string {
	u, err := b.store.GetDiscordUser(context.Background(), userID, b.config.Discord.GuildID)
	if err != nil {
		log.Printf("Failed to resolve username for %s: %v", userID, err)
		return userID
	}
	return u.DisplayName
}

// AskLore answers a lore question using the LLM with provided context.
func (b *Bot) AskLore(ctx context.Context, campaignID int64, question, loreContext string) (string, error) {
	prompt := summarise.BuildLoreQAPrompt(question, loreContext)

	type loreResult struct {
		Answer  string   `json:"answer"`
		Sources []string `json:"sources"`
	}

	// Shell out to claude CLI directly (same pattern as summariser)
	cmd := exec.CommandContext(ctx, "claude", "--print", "--model", "claude-opus-4-6", "--effort", "max")
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("lore QA failed: %w: %s", err, stderr.String())
	}

	output := summarise.StripCodeFences(stdout.Bytes())
	var result loreResult
	if err := json.Unmarshal(output, &result); err != nil {
		// If JSON parse fails, return raw text as answer
		return strings.TrimSpace(string(output)), nil
	}
	return result.Answer, nil
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
func NewBot(cfg *config.Config, store *storage.Store, transcriberFactory TranscriberFactory, summariser summarise.Summariser) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMembers |
		discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsGuildMessages

	b := &Bot{
		session:            dg,
		store:              store,
		config:             cfg,
		transcriberFactory: transcriberFactory,
		summariser:         summariser,
		webBaseURL:         resolveBaseURL(cfg),
	}

	return b, nil
}

// resolveBaseURL returns the public web URL for notification links.
// Uses web.base_url from config if set, otherwise falls back to localhost.
func resolveBaseURL(cfg *config.Config) string {
	if cfg.Web.BaseURL != "" {
		return strings.TrimRight(cfg.Web.BaseURL, "/")
	}
	return "http://localhost" + cfg.Web.ListenAddr
}

// getDiarizer returns the speaker diarizer, initializing it on first use.
func (b *Bot) getDiarizer() *diarize.Diarizer {
	b.diarizerOnce.Do(func() {
		modelDir := b.config.Diarize.ModelDir
		if modelDir == "" {
			modelDir = b.config.Transcribe.ModelDir
		}
		threads := b.config.Diarize.Threads
		if threads == 0 {
			threads = b.config.Transcribe.Threads
		}
		d, err := diarize.NewDiarizer(modelDir, threads)
		if err != nil {
			log.Printf("Failed to initialize diarizer: %v", err)
			return
		}
		b.diarizer = d
	})
	return b.diarizer
}

// SetTelegramClient sets the Telegram client for capturing group chat messages.
func (b *Bot) SetTelegramClient(c *telegram.Client) {
	b.telegramClient = c
}

// SetTTSSynthesizer sets the TTS synthesizer for voice-cloned recap playback.
func (b *Bot) SetTTSSynthesizer(synth interface {
	Synthesize(ctx context.Context, text string, refAudio []float32, refSampleRate int, refText string) ([]float32, int, error)
	SetProgressCallback(fn func(float64))
}) {
	b.ttsSynth = synth
}

// SetEmbedder sets the embedding generator for RAG-powered features.
func (b *Bot) SetEmbedder(e embed.Embedder) {
	b.embedder = e
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

	b.SyncGuildMembers()

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
	if b.diarizer != nil {
		b.diarizer.Close()
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
	rec := b.recorder
	b.mu.Unlock()

	if vc == nil {
		return
	}

	// Only care about events in the guild we are recording in.
	if vsu.GuildID != vc.GuildID {
		return
	}

	// A user JOINED our channel (either fresh join or moved in from elsewhere).
	botUserID := s.State.User.ID
	if vsu.ChannelID == channelID && vsu.UserID != botUserID {
		if rec != nil {
			beforeInOurChannel := vsu.BeforeUpdate != nil && vsu.BeforeUpdate.ChannelID == channelID
			if !beforeInOurChannel {
				rec.RecordVoiceStateJoin(vsu.UserID)
			}
		}
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

	// Clear the recorded VSU join time so a subsequent rejoin is captured
	// fresh rather than being ignored as a duplicate observation.
	if rec != nil && vsu.UserID != botUserID {
		rec.RecordVoiceStateLeave(vsu.UserID)
	}

	// Check if any non-bot users remain.
	guild, err := s.State.Guild(vc.GuildID)
	if err != nil {
		log.Printf("Failed to fetch guild state: %v", err)
		return
	}

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

	result := b.stopRecording()
	b.releaseTranscriber() // release live transcription reference

	if sessionID != 0 {
		ctx := context.Background()
		if err := b.store.EndSession(ctx, sessionID); err != nil {
			log.Printf("EndSession error (auto-stop): %v", err)
		}
		go b.runPipeline(sessionID, result)
	}
}

// stopResult bundles the results of stopping a recording session.
type stopResult struct {
	UserFiles        map[string]string
	TelegramMsgs     []telegram.Message
	PreTranscribed   map[string][]transcribe.Segment // segments already transcribed during the session
	ProcessedOffsets map[string]int64                // byte offsets already processed per user
}

// stopRecording stops the recorder, Telegram listener, and disconnects from
// voice, returning audio files and captured Telegram messages before clearing
// state. Caller must NOT hold b.mu.
func (b *Bot) stopRecording() stopResult {
	b.mu.Lock()
	defer b.mu.Unlock()

	var result stopResult

	if b.recorder != nil {
		log.Println("Stopping recorder...")
		if err := b.recorder.Stop(); err != nil {
			log.Printf("Error stopping recorder: %v", err)
		}
		result.UserFiles = b.recorder.UserFiles()
		b.recorder = nil
		log.Println("Recorder stopped")
	}
	// Wait for the live worker to finish processing any in-flight chunks
	// before we release the transcriber — it shares the same ONNX model.
	lw := b.liveWorker
	if lw != nil {
		log.Println("Waiting for live worker to drain...")
		b.mu.Unlock()
		if lw.WaitTimeout(5 * time.Second) {
			log.Println("Live worker drained")
		} else {
			log.Println("Live worker drain timed out after 5s — abandoning in-flight chunks")
		}
		b.mu.Lock()
	}
	it := b.incrementalTranscriber
	if it != nil {
		log.Println("Stopping incremental transcriber...")
		b.mu.Unlock()
		it.Stop()
		b.mu.Lock()
		result.PreTranscribed, result.ProcessedOffsets = it.CollectedSegments()
		log.Printf("Incremental transcriber: %d users pre-transcribed", len(result.PreTranscribed))
		b.incrementalTranscriber = nil
	}
	if b.telegramListener != nil {
		log.Println("Stopping Telegram listener...")
		result.TelegramMsgs = b.telegramListener.Stop()
		b.telegramListener = nil
		log.Println("Telegram listener stopped")
	}
	if b.activeVC != nil {
		log.Println("Disconnecting from voice...")
		if err := b.activeVC.Disconnect(context.Background()); err != nil {
			log.Printf("Error disconnecting from voice: %v", err)
		}
		b.activeVC = nil
		log.Println("Voice disconnected")
	}
	b.activeChannelID = ""
	b.sessionID = 0
	b.liveWorker = nil

	return result
}

package bot

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/transcribe"
	"discord-rpg-summariser/internal/voice"

	"github.com/bwmarrin/discordgo"
)

// handleInteraction is the top-level interaction dispatcher registered with the
// Discord session. It routes to the correct handler based on command name and
// subcommand.
func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()
	if len(data.Options) == 0 {
		return
	}
	sub := data.Options[0]

	switch data.Name {
	case "session":
		switch sub.Name {
		case "start":
			b.handleSessionStart(s, i)
		case "stop":
			b.handleSessionStop(s, i)
		case "status":
			b.handleSessionStatus(s, i)
		}
	case "character":
		switch sub.Name {
		case "set":
			b.handleCharacterSet(s, i)
		case "list":
			b.handleCharacterList(s, i)
		case "remove":
			b.handleCharacterRemove(s, i)
		}
	}
}

// respond sends an ephemeral or normal interaction response.
func respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
}

// respondEphemeral sends a response only visible to the invoking user.
func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// ---------------------------------------------------------------------------
// Session handlers
// ---------------------------------------------------------------------------

func (b *Bot) handleSessionStart(s *discordgo.Session, i *discordgo.InteractionCreate) {
	guildID := i.GuildID

	// Resolve the invoking user's voice state.
	guild, err := s.State.Guild(guildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to look up server information.")
		return
	}

	var userVoiceChannelID string
	for _, vs := range guild.VoiceStates {
		if vs.UserID == interactionUserID(i) {
			userVoiceChannelID = vs.ChannelID
			break
		}
	}
	if userVoiceChannelID == "" {
		respondEphemeral(s, i, "You must be in a voice channel to start a session.")
		return
	}

	// Ensure no session is already active.
	b.mu.Lock()
	if b.recorder != nil {
		b.mu.Unlock()
		respondEphemeral(s, i, "A recording session is already active.")
		return
	}
	b.mu.Unlock()

	ctx := context.Background()
	active, err := b.store.GetActiveSession(ctx, guildID)
	if err != nil {
		respondEphemeral(s, i, "Database error checking for active session.")
		return
	}
	if active != nil {
		respondEphemeral(s, i, "A recording session is already active.")
		return
	}

	// Create session directory and DB row.
	audioDir := filepath.Join(b.config.Storage.AudioDir, guildID, fmt.Sprintf("%d", time.Now().Unix()))
	sessionID, err := b.store.CreateSession(ctx, guildID, userVoiceChannelID, audioDir)
	if err != nil {
		respondEphemeral(s, i, "Failed to create session in database.")
		log.Printf("CreateSession error: %v", err)
		return
	}

	// Join voice channel.
	vc, err := s.ChannelVoiceJoin(ctx, guildID, userVoiceChannelID, false, false)
	if err != nil {
		respondEphemeral(s, i, "Failed to join your voice channel.")
		log.Printf("VoiceJoin error: %v", err)
		return
	}

	liveCh := make(chan voice.ChunkReady, 16)
	rec := voice.NewRecorder(audioDir, guildID, liveCh)
	liveWorker := voice.NewLiveWorker(b.transcriber, liveCh)
	go liveWorker.Run(ctx)

	rec.Start(vc, func(userID string) string {
		member, err := s.GuildMember(guildID, userID)
		if err != nil {
			return userID
		}
		if member.Nick != "" {
			return member.Nick
		}
		if member.User != nil {
			if member.User.GlobalName != "" {
				return member.User.GlobalName
			}
			return member.User.Username
		}
		return userID
	})

	b.mu.Lock()
	b.activeVC = vc
	b.activeChannelID = userVoiceChannelID
	b.recorder = rec
	b.sessionID = sessionID
	b.liveWorker = liveWorker
	b.mu.Unlock()

	respond(s, i, fmt.Sprintf("Recording started (session #%d). Use `/session stop` when finished.", sessionID))
}

func (b *Bot) handleSessionStop(s *discordgo.Session, i *discordgo.InteractionCreate) {
	b.mu.Lock()
	rec := b.recorder
	sessionID := b.sessionID
	b.mu.Unlock()

	if rec == nil {
		respondEphemeral(s, i, "No active recording session.")
		return
	}

	// Stop recording and disconnect; get user WAV files before state is cleared.
	userFiles := b.stopRecording()

	// Mark session as ended in DB.
	ctx := context.Background()
	if err := b.store.EndSession(ctx, sessionID); err != nil {
		log.Printf("EndSession error: %v", err)
	}

	respond(s, i, fmt.Sprintf("Recording stopped (session #%d). Processing transcript and summary...", sessionID))

	// Kick off async pipeline.
	go b.runPipeline(sessionID, userFiles)
}

func (b *Bot) handleSessionStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	active, err := b.store.GetActiveSession(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Database error.")
		return
	}

	if active == nil {
		respondEphemeral(s, i, "No active session.")
		return
	}

	dur := time.Since(active.StartedAt).Truncate(time.Second)
	respond(s, i, fmt.Sprintf("Session #%d | Status: %s | Duration: %s | Channel: <#%s>",
		active.ID, active.Status, dur, active.ChannelID))
}

// ---------------------------------------------------------------------------
// Character handlers
// ---------------------------------------------------------------------------

func (b *Bot) handleCharacterSet(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := subcommandOptions(i)

	name := opts["name"].StringValue()
	targetUserID := interactionUserID(i)
	if u, ok := opts["user"]; ok {
		targetUserID = u.UserValue(s).ID
	}

	err := b.store.SetCharacterMapping(context.Background(), storage.CharacterMapping{
		UserID:        targetUserID,
		GuildID:       i.GuildID,
		CharacterName: name,
	})
	if err != nil {
		respondEphemeral(s, i, "Failed to save character mapping.")
		log.Printf("SetCharacterMapping error: %v", err)
		return
	}

	respond(s, i, fmt.Sprintf("<@%s> is now **%s**.", targetUserID, name))
}

func (b *Bot) handleCharacterList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	mappings, err := b.store.GetCharacterMappings(context.Background(), i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to fetch character mappings.")
		return
	}

	if len(mappings) == 0 {
		respondEphemeral(s, i, "No character mappings set. Use `/character set` to add one.")
		return
	}

	var sb strings.Builder
	sb.WriteString("**Character Mappings**\n")
	for _, m := range mappings {
		sb.WriteString(fmt.Sprintf("  <@%s> -> %s\n", m.UserID, m.CharacterName))
	}

	respond(s, i, sb.String())
}

func (b *Bot) handleCharacterRemove(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := subcommandOptions(i)
	targetUserID := interactionUserID(i)
	if u, ok := opts["user"]; ok {
		targetUserID = u.UserValue(s).ID
	}

	err := b.store.DeleteCharacterMapping(context.Background(), targetUserID, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to remove character mapping.")
		log.Printf("DeleteCharacterMapping error: %v", err)
		return
	}

	respond(s, i, fmt.Sprintf("Removed character mapping for <@%s>.", targetUserID))
}

// ---------------------------------------------------------------------------
// Pipeline
// ---------------------------------------------------------------------------

// runPipeline is executed asynchronously after recording stops. It transcribes
// each user's audio, merges segments chronologically, summarises the transcript,
// persists everything to the database, and posts a notification.
func (b *Bot) runPipeline(sessionID int64, userFiles map[string]string) {
	ctx := context.Background()

	session, err := b.store.GetSession(ctx, sessionID)
	if err != nil {
		log.Printf("pipeline: GetSession(%d): %v", sessionID, err)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		return
	}

	if len(userFiles) == 0 {
		log.Printf("pipeline: no user audio files for session %d", sessionID)
		b.store.UpdateSessionSummary(ctx, sessionID, "No audio was recorded.", nil)
		b.sendNotification(sessionID, "No audio was recorded.")
		return
	}

	// Transcribe each user's WAV.
	b.store.UpdateSessionStatus(ctx, sessionID, "transcribing")

	userSegments := make(map[string][]transcribe.Segment, len(userFiles))
	for userID, wavPath := range userFiles {
		segments, err := b.transcriber.TranscribeFile(ctx, wavPath)
		if err != nil {
			log.Printf("pipeline: transcribe user %s: %v", userID, err)
			continue
		}
		userSegments[userID] = segments
	}

	if len(userSegments) == 0 {
		log.Printf("pipeline: all transcriptions failed for session %d", sessionID)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		b.sendNotification(sessionID, "Transcription failed for all users.")
		return
	}

	// Resolve character names.
	charNames := make(map[string]string, len(userSegments))
	for userID := range userSegments {
		name, err := b.store.GetCharacterName(ctx, userID, session.GuildID)
		if err != nil {
			log.Printf("pipeline: GetCharacterName(%s): %v", userID, err)
		}
		if name != "" {
			charNames[userID] = name
		}
	}

	// Merge and format.
	merged := transcribe.MergeTranscripts(userSegments, charNames)
	transcript := transcribe.FormatTranscript(merged)

	// Persist transcript segments.
	var dbSegments []storage.TranscriptSegment
	for _, seg := range merged {
		var charPtr *string
		if seg.CharacterName != "" {
			c := seg.CharacterName
			charPtr = &c
		}
		dbSegments = append(dbSegments, storage.TranscriptSegment{
			SessionID:     sessionID,
			UserID:        seg.UserID,
			CharacterName: charPtr,
			StartTime:     seg.StartTime,
			EndTime:       seg.EndTime,
			Text:          seg.Text,
		})
	}
	if err := b.store.InsertSegments(ctx, dbSegments); err != nil {
		log.Printf("pipeline: InsertSegments: %v", err)
	}

	// Summarise.
	b.store.UpdateSessionStatus(ctx, sessionID, "summarising")

	result, err := b.summariser.Summarise(ctx, transcript, "")
	if err != nil {
		log.Printf("pipeline: summarise: %v", err)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		b.sendNotification(sessionID, "Summarisation failed.")
		return
	}

	// Persist summary.
	if err := b.store.UpdateSessionSummary(ctx, sessionID, result.Summary, result.KeyEvents); err != nil {
		log.Printf("pipeline: UpdateSessionSummary: %v", err)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		return
	}

	b.sendNotification(sessionID, result.Summary)
}

// sendNotification posts an embed to the configured notification channel with a
// summary preview and a link to the full web view.
func (b *Bot) sendNotification(sessionID int64, summary string) {
	channelID := b.config.Discord.NotificationChannel
	if channelID == "" {
		return
	}

	preview := summary
	if len(preview) > 1024 {
		preview = preview[:1021] + "..."
	}

	webURL := fmt.Sprintf("%s/sessions/%d", b.webBaseURL, sessionID)

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Session #%d Summary", sessionID),
		Description: preview,
		URL:         webURL,
		Color:       0x7C3AED,
		Footer:      &discordgo.MessageEmbedFooter{Text: "View full transcript and summary on the web."},
	}

	_, err := b.session.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		log.Printf("sendNotification: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// interactionUserID returns the user ID for the person who invoked the
// interaction, handling both guild member and DM contexts.
func interactionUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil {
		return i.Member.User.ID
	}
	return i.User.ID
}

// subcommandOptions returns the option map of the first subcommand.
func subcommandOptions(i *discordgo.InteractionCreate) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	data := i.ApplicationCommandData()
	sub := data.Options[0]
	m := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(sub.Options))
	for _, opt := range sub.Options {
		m[opt.Name] = opt
	}
	return m
}

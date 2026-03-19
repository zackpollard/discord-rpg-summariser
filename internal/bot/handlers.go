package bot

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
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
	case "campaign":
		switch sub.Name {
		case "create":
			b.handleCampaignCreate(s, i)
		case "list":
			b.handleCampaignList(s, i)
		case "set":
			b.handleCampaignSet(s, i)
		case "dm":
			b.handleCampaignDM(s, i)
		case "recap":
			b.handleCampaignRecap(s, i)
		}
	case "quest":
		switch sub.Name {
		case "list":
			b.handleQuestList(s, i)
		case "complete":
			b.handleQuestComplete(s, i)
		case "fail":
			b.handleQuestFail(s, i)
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

	// Resolve the active campaign for this guild.
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, guildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		log.Printf("GetOrCreateActiveCampaign error: %v", err)
		return
	}

	// Create session directory and DB row.
	audioDir := filepath.Join(b.config.Storage.AudioDir, guildID, fmt.Sprintf("%d", time.Now().Unix()))
	sessionID, err := b.store.CreateSession(ctx, guildID, campaign.ID, userVoiceChannelID, audioDir)
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

	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		log.Printf("GetOrCreateActiveCampaign error: %v", err)
		return
	}

	err = b.store.SetCharacterMapping(ctx, storage.CharacterMapping{
		UserID:        targetUserID,
		GuildID:       i.GuildID,
		CampaignID:    campaign.ID,
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
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		log.Printf("GetOrCreateActiveCampaign error: %v", err)
		return
	}

	mappings, err := b.store.GetCharacterMappings(ctx, campaign.ID)
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

	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		log.Printf("GetOrCreateActiveCampaign error: %v", err)
		return
	}

	err = b.store.DeleteCharacterMapping(ctx, targetUserID, campaign.ID)
	if err != nil {
		respondEphemeral(s, i, "Failed to remove character mapping.")
		log.Printf("DeleteCharacterMapping error: %v", err)
		return
	}

	respond(s, i, fmt.Sprintf("Removed character mapping for <@%s>.", targetUserID))
}

// ---------------------------------------------------------------------------
// Campaign handlers
// ---------------------------------------------------------------------------

func (b *Bot) handleCampaignCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := subcommandOptions(i)
	name := opts["name"].StringValue()
	var description string
	if d, ok := opts["description"]; ok {
		description = d.StringValue()
	}

	ctx := context.Background()
	campaignID, err := b.store.CreateCampaign(ctx, i.GuildID, name, description)
	if err != nil {
		respondEphemeral(s, i, "Failed to create campaign.")
		log.Printf("CreateCampaign error: %v", err)
		return
	}

	// Auto-set as active if it's the first campaign for this guild.
	campaigns, err := b.store.ListCampaigns(ctx, i.GuildID)
	if err == nil && len(campaigns) == 1 {
		_ = b.store.SetActiveCampaign(ctx, i.GuildID, campaignID)
	}

	respond(s, i, fmt.Sprintf("Campaign **%s** created (ID %d).", name, campaignID))
}

func (b *Bot) handleCampaignList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaigns, err := b.store.ListCampaigns(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to list campaigns.")
		log.Printf("ListCampaigns error: %v", err)
		return
	}

	if len(campaigns) == 0 {
		respondEphemeral(s, i, "No campaigns yet. Use `/campaign create` to add one.")
		return
	}

	var sb strings.Builder
	sb.WriteString("**Campaigns**\n")
	for _, c := range campaigns {
		marker := "  "
		if c.IsActive {
			marker = "\u2713 "
		}
		line := fmt.Sprintf("%s**%s**", marker, c.Name)
		if c.Description != "" {
			line += fmt.Sprintf(" — %s", c.Description)
		}
		sb.WriteString(line + "\n")
	}

	respond(s, i, sb.String())
}

func (b *Bot) handleCampaignSet(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := subcommandOptions(i)
	name := opts["name"].StringValue()

	ctx := context.Background()
	campaigns, err := b.store.ListCampaigns(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to list campaigns.")
		log.Printf("ListCampaigns error: %v", err)
		return
	}

	var matched *storage.Campaign
	lower := strings.ToLower(name)
	for idx := range campaigns {
		if strings.ToLower(campaigns[idx].Name) == lower {
			matched = &campaigns[idx]
			break
		}
	}

	if matched == nil {
		respondEphemeral(s, i, fmt.Sprintf("No campaign found with name **%s**.", name))
		return
	}

	if err := b.store.SetActiveCampaign(ctx, i.GuildID, matched.ID); err != nil {
		respondEphemeral(s, i, "Failed to set active campaign.")
		log.Printf("SetActiveCampaign error: %v", err)
		return
	}

	respond(s, i, fmt.Sprintf("Active campaign set to **%s**.", matched.Name))
}

func (b *Bot) handleCampaignDM(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		return
	}

	opts := subcommandOptions(i)
	dmUserID := interactionUserID(i)
	if u, ok := opts["user"]; ok {
		dmUserID = u.UserValue(s).ID
	}

	if err := b.store.SetCampaignDM(ctx, campaign.ID, dmUserID); err != nil {
		respondEphemeral(s, i, "Failed to set DM.")
		log.Printf("SetCampaignDM error: %v", err)
		return
	}

	respond(s, i, fmt.Sprintf("<@%s> is now the DM for **%s**.", dmUserID, campaign.Name))
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
		name, err := b.store.GetCharacterName(ctx, userID, session.CampaignID)
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

	// Persist transcript segments (store only user_id; character names are
	// resolved from mappings at display time so they stay up to date).
	var dbSegments []storage.TranscriptSegment
	for _, seg := range merged {
		dbSegments = append(dbSegments, storage.TranscriptSegment{
			SessionID: sessionID,
			UserID:    seg.UserID,
			StartTime: seg.StartTime,
			EndTime:   seg.EndTime,
			Text:      seg.Text,
		})
	}
	if err := b.store.InsertSegments(ctx, dbSegments); err != nil {
		log.Printf("pipeline: InsertSegments: %v", err)
	}

	// Resolve DM name for the summariser prompt.
	dmName := ""
	campaign, _ := b.store.GetCampaign(ctx, session.CampaignID)
	if campaign != nil && campaign.DMUserID != nil {
		// Use character name if mapped, otherwise Discord display name
		if cn, _ := b.store.GetCharacterName(ctx, *campaign.DMUserID, campaign.ID); cn != "" {
			dmName = cn
		} else {
			dmName = b.ResolveUsername(*campaign.DMUserID)
		}
	}

	// Summarise.
	b.store.UpdateSessionStatus(ctx, sessionID, "summarising")

	result, err := b.summariser.Summarise(ctx, transcript, "", dmName)
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

	// Extract entities for the knowledge base (non-fatal on error).
	b.extractEntities(ctx, session, sessionID, transcript, result.Summary, dmName)

	// Extract quests (non-fatal on error).
	b.extractQuests(ctx, session, sessionID, transcript, result.Summary, dmName)
}

func (b *Bot) extractEntities(ctx context.Context, session *storage.Session, sessionID int64, transcript, summary, dmName string) {
	extractor, ok := b.summariser.(summarise.EntityExtractor)
	if !ok {
		return
	}

	existing, _ := b.store.ListEntities(ctx, session.CampaignID, "", "", 1000, 0)
	var existingNames []string
	for _, e := range existing {
		existingNames = append(existingNames, fmt.Sprintf("%s (%s)", e.Name, e.Type))
	}

	extraction, err := extractor.ExtractEntities(ctx, transcript, summary, existingNames, dmName)
	if err != nil {
		log.Printf("pipeline: entity extraction: %v", err)
		return
	}

	// Persist entities and notes
	entityIDs := make(map[string]int64) // "name|type" -> ID
	for _, e := range extraction.Entities {
		id, err := b.store.UpsertEntity(ctx, session.CampaignID, e.Name, e.Type, e.Description)
		if err != nil {
			log.Printf("pipeline: upsert entity %q: %v", e.Name, err)
			continue
		}
		entityIDs[e.Name+"|"+e.Type] = id
		if e.Notes != "" {
			if err := b.store.AddEntityNote(ctx, id, sessionID, e.Notes); err != nil {
				log.Printf("pipeline: add note for %q: %v", e.Name, err)
			}
		}
	}

	// Persist relationships
	for _, r := range extraction.Relationships {
		sourceID := findEntityID(entityIDs, r.Source)
		targetID := findEntityID(entityIDs, r.Target)
		if sourceID == 0 || targetID == 0 {
			continue
		}
		sid := sessionID
		if err := b.store.UpsertEntityRelationship(ctx, session.CampaignID, sourceID, targetID, r.Relationship, r.Description, &sid); err != nil {
			log.Printf("pipeline: upsert relationship %q->%q: %v", r.Source, r.Target, err)
		}
	}

	log.Printf("pipeline: extracted %d entities, %d relationships", len(extraction.Entities), len(extraction.Relationships))
}

func findEntityID(ids map[string]int64, name string) int64 {
	// Try to find by name with any type
	for key, id := range ids {
		if strings.HasPrefix(key, name+"|") {
			return id
		}
	}
	return 0
}

func (b *Bot) extractQuests(ctx context.Context, session *storage.Session, sessionID int64, transcript, summary, dmName string) {
	extractor, ok := b.summariser.(summarise.QuestExtractor)
	if !ok {
		return
	}

	existing, _ := b.store.ListQuests(ctx, session.CampaignID, "")
	var existingNames []string
	for _, q := range existing {
		existingNames = append(existingNames, q.Name)
	}

	extraction, err := extractor.ExtractQuests(ctx, transcript, summary, existingNames, dmName)
	if err != nil {
		log.Printf("pipeline: quest extraction: %v", err)
		return
	}

	for _, q := range extraction.Quests {
		questID, err := b.store.UpsertQuest(ctx, session.CampaignID, q.Name, q.Description, q.Status, q.Giver)
		if err != nil {
			log.Printf("pipeline: upsert quest %q: %v", q.Name, err)
			continue
		}
		var newStatus *string
		if q.Status == "completed" || q.Status == "failed" {
			newStatus = &q.Status
			if err := b.store.UpdateQuestStatus(ctx, questID, q.Status); err != nil {
				log.Printf("pipeline: update quest status %q: %v", q.Name, err)
			}
		}
		if q.Update != "" {
			if err := b.store.AddQuestUpdate(ctx, questID, sessionID, q.Update, newStatus); err != nil {
				log.Printf("pipeline: add quest update for %q: %v", q.Name, err)
			}
		}
	}

	log.Printf("pipeline: extracted %d quests", len(extraction.Quests))
}

// ---------------------------------------------------------------------------
// Quest handlers
// ---------------------------------------------------------------------------

func (b *Bot) handleQuestList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		log.Printf("GetOrCreateActiveCampaign error: %v", err)
		return
	}

	quests, err := b.store.ListQuests(ctx, campaign.ID, "")
	if err != nil {
		respondEphemeral(s, i, "Failed to list quests.")
		log.Printf("ListQuests error: %v", err)
		return
	}

	if len(quests) == 0 {
		respondEphemeral(s, i, "No quests tracked yet. Quests are automatically extracted from session transcripts.")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Quests for %s**\n", campaign.Name))
	for _, q := range quests {
		icon := "  "
		switch q.Status {
		case "active":
			icon = "\u2694\ufe0f "
		case "completed":
			icon = "\u2705 "
		case "failed":
			icon = "\u274c "
		}
		sb.WriteString(fmt.Sprintf("%s**%s** [%s]", icon, q.Name, q.Status))
		if q.Giver != "" {
			sb.WriteString(fmt.Sprintf(" — from %s", q.Giver))
		}
		sb.WriteString("\n")
		if q.Description != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", q.Description))
		}
	}

	respond(s, i, sb.String())
}

func (b *Bot) handleQuestComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	b.handleQuestStatusChange(s, i, "completed")
}

func (b *Bot) handleQuestFail(s *discordgo.Session, i *discordgo.InteractionCreate) {
	b.handleQuestStatusChange(s, i, "failed")
}

func (b *Bot) handleQuestStatusChange(s *discordgo.Session, i *discordgo.InteractionCreate, newStatus string) {
	opts := subcommandOptions(i)
	name := opts["name"].StringValue()

	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		log.Printf("GetOrCreateActiveCampaign error: %v", err)
		return
	}

	quests, err := b.store.ListQuests(ctx, campaign.ID, "")
	if err != nil {
		respondEphemeral(s, i, "Failed to list quests.")
		log.Printf("ListQuests error: %v", err)
		return
	}

	var matched *storage.Quest
	lower := strings.ToLower(name)
	for idx := range quests {
		if strings.ToLower(quests[idx].Name) == lower {
			matched = &quests[idx]
			break
		}
	}

	if matched == nil {
		respondEphemeral(s, i, fmt.Sprintf("No quest found with name **%s**.", name))
		return
	}

	if err := b.store.UpdateQuestStatus(ctx, matched.ID, newStatus); err != nil {
		respondEphemeral(s, i, "Failed to update quest status.")
		log.Printf("UpdateQuestStatus error: %v", err)
		return
	}

	respond(s, i, fmt.Sprintf("Quest **%s** marked as **%s**.", matched.Name, newStatus))
}

// ---------------------------------------------------------------------------
// Campaign recap handler
// ---------------------------------------------------------------------------

func (b *Bot) handleCampaignRecap(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		log.Printf("GetOrCreateActiveCampaign error: %v", err)
		return
	}

	// If a recap already exists, show it.
	if campaign.Recap != "" {
		respond(s, i, fmt.Sprintf("**The Story So Far — %s**\n\n%s", campaign.Name, campaign.Recap))
		return
	}

	generator, ok := b.summariser.(summarise.RecapGenerator)
	if !ok {
		respondEphemeral(s, i, "Recap generation is not supported by the current LLM backend.")
		return
	}

	// Fetch all completed sessions for this campaign.
	sessions, err := b.store.ListSessions(ctx, i.GuildID, campaign.ID, 1000, 0)
	if err != nil {
		respondEphemeral(s, i, "Failed to fetch sessions.")
		log.Printf("ListSessions error: %v", err)
		return
	}

	// Collect summaries in chronological order (ListSessions returns DESC).
	var summaries []string
	for idx := len(sessions) - 1; idx >= 0; idx-- {
		sess := sessions[idx]
		if sess.Status == "complete" && sess.Summary != nil && *sess.Summary != "" {
			summaries = append(summaries, *sess.Summary)
		}
	}

	if len(summaries) == 0 {
		respondEphemeral(s, i, "No completed sessions with summaries found. Run some sessions first!")
		return
	}

	// Acknowledge — recap generation may take a while.
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Resolve DM name.
	dmName := ""
	if campaign.DMUserID != nil {
		if cn, _ := b.store.GetCharacterName(ctx, *campaign.DMUserID, campaign.ID); cn != "" {
			dmName = cn
		} else {
			dmName = b.ResolveUsername(*campaign.DMUserID)
		}
	}

	result, err := generator.GenerateRecap(ctx, summaries, dmName)
	if err != nil {
		log.Printf("GenerateRecap error: %v", err)
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Failed to generate recap.",
		})
		return
	}

	// Persist recap.
	if err := b.store.UpdateCampaignRecap(ctx, campaign.ID, result.Recap); err != nil {
		log.Printf("UpdateCampaignRecap error: %v", err)
	}

	recap := result.Recap
	content := fmt.Sprintf("**The Story So Far — %s**\n\n%s", campaign.Name, recap)
	// Discord message limit is 2000 characters.
	if len(content) > 2000 {
		content = content[:1997] + "..."
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
	})
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

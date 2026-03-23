package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
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
		case "shared-mic":
			b.handleCampaignSharedMic(s, i)
		case "enroll":
			b.handleCampaignEnroll(s, i)
		case "telegram-dm":
			b.handleCampaignTelegramDM(s, i)
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
	audioDir := fmt.Sprintf("%s/%s/%d", b.config.Storage.AudioDir, guildID, time.Now().Unix())
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		respondEphemeral(s, i, "Failed to create audio directory.")
		log.Printf("MkdirAll %s: %v", audioDir, err)
		return
	}
	sessionID, err := b.store.CreateSession(ctx, guildID, campaign.ID, userVoiceChannelID, audioDir)
	if err != nil {
		respondEphemeral(s, i, "Failed to create session in database.")
		log.Printf("CreateSession error: %v", err)
		return
	}

	// Join voice channel.
	log.Printf("Joining voice channel %s in guild %s", userVoiceChannelID, guildID)
	vc, err := s.ChannelVoiceJoin(ctx, guildID, userVoiceChannelID, false, false)
	if err != nil {
		respondEphemeral(s, i, "Failed to join your voice channel.")
		log.Printf("VoiceJoin error: %v", err)
		return
	}
	log.Printf("Voice connection established (OpusRecv=%v)", vc.OpusRecv != nil)

	liveCh := make(chan voice.ChunkReady, 16)
	rec := voice.NewRecorder(audioDir, guildID, liveCh)
	liveWorker := voice.NewLiveWorker(b.transcriber, liveCh)

	// Configure shared mic support for live transcription.
	b.configureLiveSharedMics(ctx, liveWorker, campaign.ID)

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
	if b.telegramClient != nil && b.config.Telegram.ChatID != 0 {
		b.telegramListener = b.telegramClient.StartListening(ctx, b.config.Telegram.ChatID)
	}
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

	// Stop recording and disconnect; get user WAV files and Telegram messages.
	result := b.stopRecording()

	// Mark session as ended in DB.
	ctx := context.Background()
	if err := b.store.EndSession(ctx, sessionID); err != nil {
		log.Printf("EndSession error: %v", err)
	}

	respond(s, i, fmt.Sprintf("Recording stopped (session #%d). Processing transcript and summary...", sessionID))

	// Kick off async pipeline.
	go b.runPipeline(sessionID, result.UserFiles, result.UserJoinOffsets, result.TelegramMsgs)
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

	// Ensure a PC entity exists for this character.
	if _, err := b.store.UpsertEntity(ctx, campaign.ID, name, "pc", ""); err != nil {
		log.Printf("UpsertEntity (pc) error: %v", err)
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

func (b *Bot) handleCampaignTelegramDM(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		return
	}

	opts := subcommandOptions(i)
	telegramUserID := opts["telegram_user_id"].IntValue()

	if err := b.store.SetCampaignTelegramDM(ctx, campaign.ID, telegramUserID); err != nil {
		respondEphemeral(s, i, "Failed to set Telegram DM.")
		log.Printf("SetCampaignTelegramDM error: %v", err)
		return
	}

	respond(s, i, fmt.Sprintf("Telegram DM user ID set to **%d** for **%s**.", telegramUserID, campaign.Name))
}

func (b *Bot) handleCampaignSharedMic(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		return
	}

	opts := subcommandOptions(i)
	micUser := opts["user"].UserValue(s)
	partnerOpt, hasPartner := opts["partner"]
	partnerNameOpt, hasPartnerName := opts["partner-name"]

	// Neither provided -> remove shared mic config.
	if !hasPartner && !hasPartnerName {
		if err := b.store.DeleteSharedMic(ctx, campaign.ID, micUser.ID); err != nil {
			respondEphemeral(s, i, "Failed to remove shared mic config.")
			log.Printf("DeleteSharedMic error: %v", err)
			return
		}
		respond(s, i, fmt.Sprintf("Shared mic config removed for <@%s>.", micUser.ID))
		return
	}

	// Both provided -> error.
	if hasPartner && hasPartnerName {
		respondEphemeral(s, i, "Provide either `partner` (Discord user) or `partner-name` (text), not both.")
		return
	}

	var partnerID string
	var displayMsg string

	if hasPartner {
		// Partner is a real Discord user.
		partner := partnerOpt.UserValue(s)
		partnerID = partner.ID
		displayMsg = fmt.Sprintf("Shared mic configured: <@%s>'s audio will be split between <@%s> and <@%s>.", micUser.ID, micUser.ID, partner.ID)
	} else {
		// Partner is a non-Discord person -- use synthetic ID and auto-create character mapping.
		partnerName := partnerNameOpt.StringValue()
		partnerID = storage.SyntheticPartnerID(micUser.ID)
		if err := b.store.SetCharacterMapping(ctx, storage.CharacterMapping{
			UserID:        partnerID,
			GuildID:       i.GuildID,
			CampaignID:    campaign.ID,
			CharacterName: partnerName,
		}); err != nil {
			log.Printf("SetCharacterMapping for partner error: %v", err)
		}
		displayMsg = fmt.Sprintf("Shared mic configured: <@%s>'s audio will be split between <@%s> and **%s**.", micUser.ID, micUser.ID, partnerName)
	}

	if err := b.store.SetSharedMic(ctx, campaign.ID, micUser.ID, partnerID); err != nil {
		respondEphemeral(s, i, "Failed to save shared mic config.")
		log.Printf("SetSharedMic error: %v", err)
		return
	}

	respond(s, i, displayMsg)
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

	// Check for optional "last" parameter.
	opts := subcommandOptions(i)
	var lastN int
	if opt, ok := opts["last"]; ok {
		lastN = int(opt.IntValue())
	}

	// If no "last" filter and a cached recap already exists, show it.
	if lastN == 0 && campaign.Recap != "" {
		respond(s, i, fmt.Sprintf("**The Story So Far — %s**\n\n%s", campaign.Name, campaign.Recap))
		return
	}

	generator, ok := b.summariser.(summarise.RecapGenerator)
	if !ok {
		respondEphemeral(s, i, "Recap generation is not supported by the current LLM backend.")
		return
	}

	var summaries []string
	if lastN > 0 {
		// Fetch only the last N complete sessions.
		sessions, err := b.store.GetLatestCompleteSessions(ctx, campaign.ID, lastN)
		if err != nil {
			respondEphemeral(s, i, "Failed to fetch sessions.")
			log.Printf("GetLatestCompleteSessions error: %v", err)
			return
		}
		for _, sess := range sessions {
			if sess.Summary != nil && *sess.Summary != "" {
				summaries = append(summaries, *sess.Summary)
			}
		}
	} else {
		// Fetch all completed sessions for this campaign.
		sessions, err := b.store.ListSessions(ctx, i.GuildID, campaign.ID, 1000, 0)
		if err != nil {
			respondEphemeral(s, i, "Failed to fetch sessions.")
			log.Printf("ListSessions error: %v", err)
			return
		}
		// Collect summaries in chronological order (ListSessions returns DESC).
		for idx := len(sessions) - 1; idx >= 0; idx-- {
			sess := sessions[idx]
			if sess.Status == "complete" && sess.Summary != nil && *sess.Summary != "" {
				summaries = append(summaries, *sess.Summary)
			}
		}
	}

	if len(summaries) == 0 {
		respondEphemeral(s, i, "No completed sessions with summaries found. Run some sessions first!")
		return
	}

	// Acknowledge -- recap generation may take a while.
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

	// Only persist recap when generating a full campaign recap (no "last" filter).
	if lastN == 0 {
		if err := b.store.UpdateCampaignRecap(ctx, campaign.ID, result.Recap); err != nil {
			log.Printf("UpdateCampaignRecap error: %v", err)
		}
	}

	recap := result.Recap
	var title string
	if lastN > 0 {
		title = fmt.Sprintf("**Recent Recap (last %d sessions) — %s**", lastN, campaign.Name)
	} else {
		title = fmt.Sprintf("**The Story So Far — %s**", campaign.Name)
	}
	content := fmt.Sprintf("%s\n\n%s", title, recap)
	// Discord message limit is 2000 characters.
	if len(content) > 2000 {
		content = content[:1997] + "..."
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
	})
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

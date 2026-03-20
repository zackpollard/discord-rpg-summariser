package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"discord-rpg-summariser/internal/audio"
	"discord-rpg-summariser/internal/diarize"
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/telegram"
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
	go b.runPipeline(sessionID, result.UserFiles, result.TelegramMsgs)
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

	// Neither provided → remove shared mic config.
	if !hasPartner && !hasPartnerName {
		if err := b.store.DeleteSharedMic(ctx, campaign.ID, micUser.ID); err != nil {
			respondEphemeral(s, i, "Failed to remove shared mic config.")
			log.Printf("DeleteSharedMic error: %v", err)
			return
		}
		respond(s, i, fmt.Sprintf("Shared mic config removed for <@%s>.", micUser.ID))
		return
	}

	// Both provided → error.
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
		// Partner is a non-Discord person — use synthetic ID and auto-create character mapping.
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
// Live shared mic support
// ---------------------------------------------------------------------------

// configureLiveSharedMics loads shared mic configs and enrollments for the
// campaign and configures the live worker to identify speakers.
func (b *Bot) configureLiveSharedMics(ctx context.Context, lw *voice.LiveWorker, campaignID int64) {
	mics, err := b.store.GetSharedMics(ctx, campaignID)
	if err != nil || len(mics) == 0 {
		return
	}

	micInfos := make(map[string]voice.SharedMicInfo, len(mics))
	for _, m := range mics {
		ownerName, _ := b.store.GetCharacterName(ctx, m.DiscordUserID, campaignID)
		if ownerName == "" {
			ownerName = b.ResolveUsername(m.DiscordUserID)
		}
		partnerName, _ := b.store.GetCharacterName(ctx, m.PartnerUserID, campaignID)
		if partnerName == "" {
			partnerName = m.PartnerUserID
		}
		micInfos[m.DiscordUserID] = voice.SharedMicInfo{
			PartnerUserID:      m.PartnerUserID,
			OwnerDisplayName:   ownerName,
			PartnerDisplayName: partnerName,
		}
	}
	lw.SetSharedMics(micInfos)

	// Set up embedding-based speaker identification if diarizer is available.
	d := b.getDiarizer()
	if d == nil {
		return
	}

	// Load enrolled embeddings for all shared mic users.
	enrollments := make(map[string][]float32)
	for _, m := range mics {
		if e, err := b.store.GetSpeakerEnrollment(ctx, campaignID, m.DiscordUserID); err == nil {
			enrollments[m.DiscordUserID] = e.Embedding
		}
		if e, err := b.store.GetSpeakerEnrollment(ctx, campaignID, m.PartnerUserID); err == nil {
			enrollments[m.PartnerUserID] = e.Embedding
		}
	}

	if len(enrollments) > 0 {
		lw.SetSpeakerIdentifier(&embeddingSpeakerIdentifier{
			diarizer:    d,
			enrollments: enrollments,
		})
		log.Printf("Live transcription: shared mic speaker identification enabled with %d enrollment(s)", len(enrollments))
	}
}

// embeddingSpeakerIdentifier implements voice.SpeakerIdentifier using
// voice enrollment embeddings and cosine similarity.
type embeddingSpeakerIdentifier struct {
	diarizer    *diarize.Diarizer
	enrollments map[string][]float32 // userID -> embedding
}

func (id *embeddingSpeakerIdentifier) IdentifySpeaker(samples []float32, mic voice.SharedMicInfo) (string, string) {
	fallback := mic.OwnerDisplayName + " & " + mic.PartnerDisplayName

	emb, err := id.diarizer.ExtractEmbedding(samples)
	if err != nil {
		return "", fallback
	}

	// Compare against all enrolled embeddings and pick the best match.
	var bestID string
	bestSim := -1.0
	for uid, enrolled := range id.enrollments {
		sim := diarize.CosineSimilarity(emb, enrolled)
		if sim > bestSim {
			bestSim = sim
			bestID = uid
		}
	}

	if bestSim < 0.3 {
		return "", fallback
	}

	if bestID == mic.PartnerUserID {
		return bestID, mic.PartnerDisplayName
	}
	return bestID, mic.OwnerDisplayName
}

// ---------------------------------------------------------------------------
// Voice enrollment
// ---------------------------------------------------------------------------

const enrollDuration = 10 * time.Second

func (b *Bot) handleCampaignEnroll(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		return
	}

	opts := subcommandOptions(i)

	// Determine the Discord user whose mic we'll record from.
	micUserID := interactionUserID(i)
	if u, ok := opts["user"]; ok {
		micUserID = u.UserValue(s).ID
	}

	// If partner flag is set, enroll the shared-mic partner instead of the
	// mic owner. The partner must be the only one speaking during the sample.
	enrollPartner := false
	if p, ok := opts["partner"]; ok {
		enrollPartner = p.BoolValue()
	}

	// Resolve who we're saving the enrollment for.
	enrollUserID := micUserID
	if enrollPartner {
		// Look up the shared mic config to find the partner's ID.
		mic, err := b.store.GetSharedMics(ctx, campaign.ID)
		if err != nil {
			respondEphemeral(s, i, "Failed to load shared mic config.")
			return
		}
		var found bool
		for _, m := range mic {
			if m.DiscordUserID == micUserID {
				enrollUserID = m.PartnerUserID
				found = true
				break
			}
		}
		if !found {
			respondEphemeral(s, i, fmt.Sprintf("No shared mic configured for <@%s>. Set one up with `/campaign shared-mic` first.", micUserID))
			return
		}
	}

	// Ensure no session is active (would conflict with the recorder).
	b.mu.Lock()
	if b.recorder != nil {
		b.mu.Unlock()
		respondEphemeral(s, i, "A recording session is active. Enrollment will happen automatically when it ends.")
		return
	}
	b.mu.Unlock()

	// Ensure the diarizer (and its embedding extractor) is available.
	d := b.getDiarizer()
	if d == nil {
		respondEphemeral(s, i, "Speaker embedding models are not available.")
		return
	}

	// Find the mic user's voice channel.
	guild, err := s.State.Guild(i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to look up server information.")
		return
	}
	var voiceChannelID string
	for _, vs := range guild.VoiceStates {
		if vs.UserID == micUserID {
			voiceChannelID = vs.ChannelID
			break
		}
	}
	if voiceChannelID == "" {
		respondEphemeral(s, i, fmt.Sprintf("<@%s> must be in a voice channel.", micUserID))
		return
	}

	// Defer the response — enrollment takes ~10 seconds.
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Run enrollment in the background.
	go b.runEnrollment(s, i, campaign.ID, micUserID, enrollUserID, enrollPartner, voiceChannelID)
}

func (b *Bot) runEnrollment(s *discordgo.Session, i *discordgo.InteractionCreate, campaignID int64, micUserID, enrollUserID string, isPartner bool, voiceChannelID string) {
	ctx := context.Background()

	// Create a temporary directory for the WAV file.
	tmpDir, err := os.MkdirTemp("", "enroll-*")
	if err != nil {
		b.enrollFollowup(s, i, "Failed to create temporary directory.")
		return
	}
	defer os.RemoveAll(tmpDir)

	// Join voice channel.
	vc, err := s.ChannelVoiceJoin(ctx, i.GuildID, voiceChannelID, false, false)
	if err != nil {
		b.enrollFollowup(s, i, "Failed to join voice channel.")
		return
	}

	// Record for the enrollment duration.
	rec := voice.NewRecorder(tmpDir, i.GuildID, nil)
	rec.Start(vc, func(userID string) string {
		member, err := s.GuildMember(i.GuildID, userID)
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

	time.Sleep(enrollDuration)

	if err := rec.Stop(); err != nil {
		log.Printf("enroll: stop recorder: %v", err)
	}
	if err := vc.Disconnect(ctx); err != nil {
		log.Printf("enroll: disconnect: %v", err)
	}

	// Find the mic user's WAV file (audio comes from their Discord account).
	userFiles := rec.UserFiles()
	wavPath, ok := userFiles[micUserID]
	if !ok {
		who := fmt.Sprintf("<@%s>", micUserID)
		if isPartner {
			who = "The partner"
		}
		b.enrollFollowup(s, i, fmt.Sprintf("%s did not speak during the enrollment window. Please try again and make sure to talk.", who))
		return
	}

	// Resample and extract embedding.
	samples, err := audio.LoadAndResample(wavPath)
	if err != nil {
		b.enrollFollowup(s, i, "Failed to process audio.")
		log.Printf("enroll: resample: %v", err)
		return
	}

	d := b.getDiarizer()
	embedding, err := d.ExtractEmbedding(samples)
	if err != nil {
		b.enrollFollowup(s, i, "Failed to extract voice embedding (was there enough speech?).")
		log.Printf("enroll: extract embedding: %v", err)
		return
	}

	// Persist the enrollment under the correct user ID.
	if err := b.store.UpsertSpeakerEnrollment(ctx, campaignID, enrollUserID, embedding); err != nil {
		b.enrollFollowup(s, i, "Failed to save voice enrollment.")
		log.Printf("enroll: save: %v", err)
		return
	}

	if isPartner {
		b.enrollFollowup(s, i, fmt.Sprintf("Voice enrollment saved for <@%s>'s shared-mic partner. Their voice will now be used for speaker identification.", micUserID))
	} else {
		b.enrollFollowup(s, i, fmt.Sprintf("Voice enrollment saved for <@%s>. Shared mic sessions will now use this to identify their voice.", enrollUserID))
	}
}

func (b *Bot) enrollFollowup(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: msg,
	})
}

// ---------------------------------------------------------------------------
// Pipeline
// ---------------------------------------------------------------------------

// runPipeline is executed asynchronously after recording stops. It transcribes
// each user's audio, merges segments chronologically (including any Telegram
// messages), summarises the transcript, persists everything to the database,
// and posts a notification.
func (b *Bot) runPipeline(sessionID int64, userFiles map[string]string, telegramMsgs []telegram.Message) {
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

	// Transcribe each user's WAV, with diarization for shared mics.
	b.store.UpdateSessionStatus(ctx, sessionID, "transcribing")

	// Load shared mic config for this campaign.
	sharedMics, _ := b.store.GetSharedMics(ctx, session.CampaignID)
	sharedMicMap := make(map[string]storage.SharedMic, len(sharedMics))
	for _, m := range sharedMics {
		sharedMicMap[m.DiscordUserID] = m
	}

	userSegments := make(map[string][]transcribe.Segment, len(userFiles))
	for userID, wavPath := range userFiles {
		if mic, ok := sharedMicMap[userID]; ok {
			// Shared mic: diarize then attribute segments.
			b.transcribeSharedMic(ctx, wavPath, mic, userSegments)
		} else {
			// Normal single-user transcription.
			segments, err := b.transcriber.TranscribeFile(ctx, wavPath)
			if err != nil {
				log.Printf("pipeline: transcribe user %s: %v", userID, err)
				continue
			}
			userSegments[userID] = segments
		}
	}

	if len(userSegments) == 0 {
		log.Printf("pipeline: all transcriptions failed for session %d", sessionID)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		b.sendNotification(sessionID, "Transcription failed for all users.")
		return
	}

	// Auto-enroll voice embeddings for non-shared-mic users so future
	// shared-mic sessions can identify speakers by voice.
	if d := b.getDiarizer(); d != nil {
		for userID, wavPath := range userFiles {
			if _, ok := sharedMicMap[userID]; ok {
				continue
			}
			samples, err := audio.LoadAndResample(wavPath)
			if err != nil {
				continue
			}
			embedding, err := d.ExtractEmbedding(samples)
			if err != nil {
				log.Printf("pipeline: auto-enroll %s: %v", userID, err)
				continue
			}
			if err := b.store.UpsertSpeakerEnrollment(ctx, session.CampaignID, userID, embedding); err != nil {
				log.Printf("pipeline: save enrollment %s: %v", userID, err)
			}
		}
		log.Printf("pipeline: auto-enrolled voice embeddings for %d user(s)", len(userFiles)-len(sharedMicMap))
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

	// Merge voice segments.
	merged := transcribe.MergeTranscripts(userSegments, charNames)

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

	// Resolve DM name and campaign for Telegram filtering.
	dmName := ""
	campaign, _ := b.store.GetCampaign(ctx, session.CampaignID)
	if campaign != nil && campaign.DMUserID != nil {
		if cn, _ := b.store.GetCharacterName(ctx, *campaign.DMUserID, campaign.ID); cn != "" {
			dmName = cn
		} else {
			dmName = b.ResolveUsername(*campaign.DMUserID)
		}
	}

	// Process and persist Telegram messages, then interleave into transcript.
	transcript := b.buildTranscriptWithTelegram(ctx, session, campaign, merged, telegramMsgs, dmName)

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

	// Extract combat encounters (non-fatal on error).
	b.extractCombat(ctx, session, sessionID, transcript, result.Summary, dmName)
}

// buildTranscriptWithTelegram persists Telegram messages to the DB, filters
// them, and returns a formatted transcript with voice segments and Telegram
// messages interleaved chronologically.
func (b *Bot) buildTranscriptWithTelegram(
	ctx context.Context,
	session *storage.Session,
	campaign *storage.Campaign,
	merged []transcribe.UserSegment,
	telegramMsgs []telegram.Message,
	dmName string,
) string {
	// If no Telegram messages, just format voice segments.
	if len(telegramMsgs) == 0 {
		return transcribe.FormatTranscript(merged)
	}

	// Determine the Telegram DM user ID for filtering.
	var telegramDMID int64
	if campaign != nil && campaign.TelegramDMUserID != nil {
		telegramDMID = *campaign.TelegramDMUserID
	}

	// Persist all Telegram messages to DB.
	var dbMsgs []storage.TelegramMessage
	for _, m := range telegramMsgs {
		isDM := telegramDMID != 0 && m.FromID == telegramDMID
		dbMsgs = append(dbMsgs, storage.TelegramMessage{
			SessionID:     session.ID,
			TelegramMsgID: m.MessageID,
			FromUserID:    m.FromID,
			FromUsername:  m.FromUsername,
			FromDisplay:   m.FromDisplay,
			Text:          m.Text,
			SentAt:        m.Timestamp,
			IsDM:          isDM,
		})
	}
	if err := b.store.InsertTelegramMessages(ctx, dbMsgs); err != nil {
		log.Printf("pipeline: InsertTelegramMessages: %v", err)
	}

	// Filter: only DM messages that pass relevance check.
	var entries []transcribe.TelegramEntry
	senderLabel := "DM"
	if dmName != "" {
		senderLabel = dmName
	}

	for _, m := range telegramMsgs {
		isDM := telegramDMID != 0 && m.FromID == telegramDMID
		if !telegram.IsRelevant(m, isDM) {
			continue
		}
		elapsed := m.Timestamp.Sub(session.StartedAt).Seconds()
		if elapsed < 0 {
			elapsed = 0
		}
		name := senderLabel
		if !isDM {
			name = m.FromDisplay
		}
		entries = append(entries, transcribe.TelegramEntry{
			ElapsedSecs: elapsed,
			SenderName:  name,
			Text:        m.Text,
		})
	}

	if len(entries) == 0 {
		return transcribe.FormatTranscript(merged)
	}

	log.Printf("pipeline: interleaving %d Telegram messages into transcript", len(entries))
	return transcribe.FormatTranscriptWithTelegram(merged, entries)
}

// transcribeSharedMic diarizes a shared-mic WAV file and attributes each
// transcription segment to the correct speaker.
func (b *Bot) transcribeSharedMic(ctx context.Context, wavPath string, mic storage.SharedMic, userSegments map[string][]transcribe.Segment) {
	d := b.getDiarizer()
	if d == nil {
		log.Printf("pipeline: diarizer not available, treating shared mic user %s as single speaker", mic.DiscordUserID)
		segments, err := b.transcriber.TranscribeFile(ctx, wavPath)
		if err != nil {
			log.Printf("pipeline: transcribe shared mic user %s: %v", mic.DiscordUserID, err)
			return
		}
		userSegments[mic.DiscordUserID] = segments
		return
	}

	// Resample to 16kHz for diarization.
	samples, err := audio.LoadAndResample(wavPath)
	if err != nil {
		log.Printf("pipeline: resample for diarization %s: %v", mic.DiscordUserID, err)
		return
	}

	// Run speaker diarization.
	diarSegments, err := d.Diarize(samples)
	if err != nil {
		log.Printf("pipeline: diarize %s: %v", mic.DiscordUserID, err)
		// Fall back to single speaker.
		segments, _ := b.transcriber.TranscribeFile(ctx, wavPath)
		if segments != nil {
			userSegments[mic.DiscordUserID] = segments
		}
		return
	}

	// Try to identify speakers using enrolled voice embeddings.
	primarySpeaker := -1
	speakers := diarize.UniqueSpeakers(diarSegments)
	if len(speakers) == 2 {
		micOwnerEnroll, _ := b.store.GetSpeakerEnrollment(ctx, mic.CampaignID, mic.DiscordUserID)
		partnerEnroll, _ := b.store.GetSpeakerEnrollment(ctx, mic.CampaignID, mic.PartnerUserID)

		if micOwnerEnroll != nil || partnerEnroll != nil {
			spk0Audio := diarize.ExtractSpeakerAudio(samples, diarSegments, speakers[0])
			spk1Audio := diarize.ExtractSpeakerAudio(samples, diarSegments, speakers[1])
			emb0, err0 := d.ExtractEmbedding(spk0Audio)
			emb1, err1 := d.ExtractEmbedding(spk1Audio)

			if err0 == nil && err1 == nil {
				var ownerEmb, partnerEmb []float32
				if micOwnerEnroll != nil {
					ownerEmb = micOwnerEnroll.Embedding
				}
				if partnerEnroll != nil {
					partnerEmb = partnerEnroll.Embedding
				}
				primarySpeaker = diarize.IdentifySpeakerByEmbedding(emb0, emb1, ownerEmb, partnerEmb)
				if primarySpeaker >= 0 {
					// Map from position (0/1) back to actual speaker ID.
					primarySpeaker = speakers[primarySpeaker]
					log.Printf("pipeline: identified speakers by voice enrollment for %s", mic.DiscordUserID)
				}
			}
		}
	}

	if primarySpeaker < 0 {
		// Fall back to speaking time heuristic.
		primarySpeaker = diarize.IdentifyPrimarySpeaker(diarSegments)
		log.Printf("pipeline: no voice enrollment, using speaking time heuristic for %s", mic.DiscordUserID)
	}
	log.Printf("pipeline: diarized %s: %d segments, mic owner is speaker %d", mic.DiscordUserID, len(diarSegments), primarySpeaker)

	// Transcribe the full audio.
	allSegments, err := b.transcriber.TranscribeFile(ctx, wavPath)
	if err != nil {
		log.Printf("pipeline: transcribe shared mic %s: %v", mic.DiscordUserID, err)
		return
	}

	// Attribute each segment to a speaker based on diarization overlap.
	for _, seg := range allSegments {
		speaker := diarize.AttributeSegment(seg.StartTime, seg.EndTime, diarSegments)
		if speaker == primarySpeaker {
			userSegments[mic.DiscordUserID] = append(userSegments[mic.DiscordUserID], seg)
		} else {
			userSegments[mic.PartnerUserID] = append(userSegments[mic.PartnerUserID], seg)
		}
	}
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

	// Collect player character names so the LLM doesn't extract them as NPCs.
	charMappings, _ := b.store.GetCharacterMappings(ctx, session.CampaignID)
	var playerCharacters []string
	for _, m := range charMappings {
		playerCharacters = append(playerCharacters, m.CharacterName)
	}

	// Ensure PC entities exist for all player characters before extraction.
	pcEntityIDs, err := b.store.EnsurePCEntities(ctx, session.CampaignID, playerCharacters)
	if err != nil {
		log.Printf("pipeline: ensure PC entities: %v", err)
	}

	extraction, err := extractor.ExtractEntities(ctx, transcript, summary, existingNames, dmName, playerCharacters)
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

	// Add PC entity IDs so relationships referencing PCs can be resolved.
	for name, id := range pcEntityIDs {
		entityIDs[name+"|pc"] = id
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

	// Link entity references to transcript segments.
	b.linkEntityReferences(ctx, sessionID, entityIDs)
}

// linkEntityReferences scans transcript segments for mentions of entities and
// inserts entity_references rows linking them.
func (b *Bot) linkEntityReferences(ctx context.Context, sessionID int64, entityIDs map[string]int64) {
	segments, err := b.store.GetTranscript(ctx, sessionID)
	if err != nil {
		log.Printf("pipeline: linkEntityReferences: get transcript: %v", err)
		return
	}
	if len(segments) == 0 {
		return
	}

	// Build a map of entity name -> entity ID, skipping names shorter than 3 chars.
	nameToID := make(map[string]int64)
	for key, id := range entityIDs {
		parts := strings.SplitN(key, "|", 2)
		name := parts[0]
		if len([]rune(name)) < 3 {
			continue
		}
		nameToID[name] = id
	}

	if len(nameToID) == 0 {
		return
	}

	var refs []storage.EntityReference
	for i := range segments {
		seg := &segments[i]
		matches := findEntityMentions(seg.Text, nameToID)
		for entityName, entityID := range matches {
			ctx := truncateContext(seg.Text, entityName, 200)
			segID := seg.ID
			refs = append(refs, storage.EntityReference{
				EntityID:  entityID,
				SessionID: sessionID,
				SegmentID: &segID,
				Context:   ctx,
			})
		}
	}

	if len(refs) == 0 {
		return
	}

	if err := b.store.InsertEntityReferences(ctx, refs); err != nil {
		log.Printf("pipeline: linkEntityReferences: insert: %v", err)
		return
	}

	log.Printf("pipeline: linked %d entity references for session %d", len(refs), sessionID)
}

// findEntityMentions performs case-insensitive word-boundary matching of entity
// names against the given text. Returns a map of matched entity name -> ID.
func findEntityMentions(text string, nameToID map[string]int64) map[string]int64 {
	matches := make(map[string]int64)
	lower := strings.ToLower(text)

	for name, id := range nameToID {
		pattern := `(?i)\b` + regexp.QuoteMeta(name) + `\b`
		re, err := regexp.Compile(pattern)
		if err != nil {
			// If the name has characters that break regex even after quoting,
			// fall back to simple case-insensitive contains with boundary check.
			if containsWordBoundary(lower, strings.ToLower(name)) {
				matches[name] = id
			}
			continue
		}
		if re.MatchString(text) {
			matches[name] = id
		}
	}

	return matches
}

// containsWordBoundary checks if text contains substr at a word boundary.
func containsWordBoundary(text, substr string) bool {
	idx := strings.Index(text, substr)
	if idx < 0 {
		return false
	}
	// Check left boundary.
	if idx > 0 {
		r := rune(text[idx-1])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	// Check right boundary.
	end := idx + len(substr)
	if end < len(text) {
		r := rune(text[end])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// truncateContext returns a context snippet from text around the entity name,
// truncated to maxLen characters.
func truncateContext(text, entityName string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	// Try to center around the entity mention.
	lower := strings.ToLower(text)
	idx := strings.Index(lower, strings.ToLower(entityName))
	if idx < 0 {
		return text[:maxLen]
	}
	start := idx - (maxLen-len(entityName))/2
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(text) {
		end = len(text)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}
	return text[start:end]
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

func (b *Bot) extractCombat(ctx context.Context, session *storage.Session, sessionID int64, transcript, summary, dmName string) {
	extractor, ok := b.summariser.(summarise.CombatExtractor)
	if !ok {
		return
	}

	// Collect player character names.
	charMappings, _ := b.store.GetCharacterMappings(ctx, session.CampaignID)
	var playerCharacters []string
	for _, m := range charMappings {
		playerCharacters = append(playerCharacters, m.CharacterName)
	}

	extraction, err := extractor.ExtractCombat(ctx, transcript, summary, dmName, playerCharacters)
	if err != nil {
		log.Printf("pipeline: combat extraction: %v", err)
		return
	}

	for _, enc := range extraction.Encounters {
		encID, err := b.store.InsertCombatEncounter(ctx, storage.CombatEncounter{
			SessionID:  sessionID,
			CampaignID: session.CampaignID,
			Name:       enc.Name,
			StartTime:  enc.StartTime,
			EndTime:    enc.EndTime,
			Summary:    enc.Summary,
		})
		if err != nil {
			log.Printf("pipeline: insert combat encounter %q: %v", enc.Name, err)
			continue
		}

		var actions []storage.CombatAction
		for _, a := range enc.Actions {
			actions = append(actions, storage.CombatAction{
				Actor:      a.Actor,
				ActionType: a.ActionType,
				Target:     a.Target,
				Detail:     a.Detail,
				Damage:     a.Damage,
				Round:      a.Round,
				Timestamp:  a.Timestamp,
			})
		}
		if err := b.store.InsertCombatActions(ctx, encID, actions); err != nil {
			log.Printf("pipeline: insert combat actions for %q: %v", enc.Name, err)
		}
	}

	log.Printf("pipeline: extracted %d combat encounters", len(extraction.Encounters))
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

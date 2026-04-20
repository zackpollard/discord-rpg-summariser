package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"discord-rpg-summariser/internal/audio"
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/tts"
	"discord-rpg-summariser/internal/voice"

	"github.com/bwmarrin/discordgo"
)

// handleCampaignGenerateRecapAudio generates a TTS WAV file for the campaign
// recap or previously-on narration and caches it for later playback.
func (b *Bot) handleCampaignGenerateRecapAudio(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if b.ttsSynth == nil {
		respondEphemeral(s, i, "TTS is not configured on this bot.")
		return
	}

	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		return
	}

	opts := subcommandOptions(i)

	source := "recap"
	if opt, ok := opts["source"]; ok {
		source = opt.StringValue()
	}

	voiceUserID, profileID := b.resolveVoiceOption(opts)
	if voiceUserID == "" && profileID == 0 {
		voiceUserID = interactionUserID(i)
	}

	// Determine the text to synthesize.
	var ttsText string
	switch source {
	case "previously-on":
		ttsText = campaign.PreviouslyOn
		if ttsText == "" {
			// Generate it on the fly if not cached.
			gen, ok := b.summariser.(summarise.PreviouslyOnGenerator)
			if !ok {
				respondEphemeral(s, i, "Previously-on generation is not supported by the current LLM backend.")
				return
			}
			sessions, err := b.store.GetLatestCompleteSessions(ctx, campaign.ID, 1)
			if err != nil || len(sessions) == 0 || sessions[0].Summary == nil {
				respondEphemeral(s, i, "No completed sessions with summaries found.")
				return
			}
			result, err := gen.GeneratePreviouslyOn(ctx, *sessions[0].Summary, campaign.Recap)
			if err != nil {
				respondEphemeral(s, i, fmt.Sprintf("Failed to generate previously-on text: %v", err))
				return
			}
			ttsText = result.Text
			_ = b.store.UpdateCampaignPreviouslyOn(ctx, campaign.ID, ttsText)
		}
	default:
		if campaign.Recap == "" {
			respondEphemeral(s, i, "No recap has been generated yet. Use `/campaign recap` first.")
			return
		}
		ttsText = campaign.Recap
	}

	// Acknowledge — generation takes a while.
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	var ref *tts.ReferenceClip
	var voiceKey string
	if profileID > 0 {
		ref, err = extractProfileRef(b.store, ctx, profileID)
		voiceKey = storage.VoiceKeyForProfile(profileID)
	} else {
		ref, err = tts.ExtractReference(b.store, campaign.ID, voiceUserID)
		voiceKey = storage.VoiceKeyForUser(voiceUserID)
	}
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to extract reference audio: %v", err),
		})
		return
	}

	sourceLabel := "recap"
	if source == "previously-on" {
		sourceLabel = "previously-on"
	}

	followupMsg, _ := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Generating %s audio... 0%%", sourceLabel),
	})

	// Set up progress reporting.
	var lastProgress atomic.Int64
	b.ttsSynth.SetProgressCallback(func(p float64) {
		lastProgress.Store(int64(p * 100))
	})
	defer b.ttsSynth.SetProgressCallback(nil)

	// Start a goroutine that edits the followup message with progress updates.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				pct := lastProgress.Load()
				if followupMsg != nil && pct > 0 && pct < 100 {
					content := fmt.Sprintf("Generating %s audio... %d%%", sourceLabel, pct)
					s.FollowupMessageEdit(i.Interaction, followupMsg.ID, &discordgo.WebhookEdit{
						Content: &content,
					})
				}
			}
		}
	}()

	samples, sampleRate, err := b.ttsSynth.Synthesize(ctx, ttsText, ref.Samples, ref.SampleRate, ref.Text)
	close(done)

	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("TTS generation failed: %v", err),
		})
		return
	}

	// Save to cache.
	cacheDir := filepath.Join(b.config.Storage.AudioDir, "tts-cache")
	os.MkdirAll(cacheDir, 0o755)
	sanitizedKey := strings.ReplaceAll(voiceKey, ":", "-")
	wavPath := filepath.Join(cacheDir, fmt.Sprintf("%s-%d-%s.wav", source, campaign.ID, sanitizedKey))

	if err := tts.WriteWAV(wavPath, samples, sampleRate); err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to save audio: %v", err),
		})
		return
	}

	_ = b.store.UpsertTTSCache(ctx, storage.TTSAudioCache{
		CampaignID: campaign.ID,
		Source:     source,
		VoiceKey:   voiceKey,
		AudioPath:  wavPath,
	})

	content := fmt.Sprintf("%s audio generated! Use `/campaign play-recap` to play it in a voice channel.", sourceLabel)
	if followupMsg != nil {
		s.FollowupMessageEdit(i.Interaction, followupMsg.ID, &discordgo.WebhookEdit{
			Content: &content,
		})
	} else {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: content})
	}
}

// handleCampaignPlayRecap plays a previously generated TTS recap in the user's
// voice channel.
func (b *Bot) handleCampaignPlayRecap(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		return
	}

	opts := subcommandOptions(i)

	source := "recap"
	if opt, ok := opts["source"]; ok {
		source = opt.StringValue()
	}

	voiceUserID, profileID := b.resolveVoiceOption(opts)
	if voiceUserID == "" && profileID == 0 {
		voiceUserID = interactionUserID(i)
	}

	var voiceKey string
	if profileID > 0 {
		voiceKey = storage.VoiceKeyForProfile(profileID)
	} else {
		voiceKey = storage.VoiceKeyForUser(voiceUserID)
	}

	// Look up cached audio from DB.
	cached, err := b.store.GetTTSCache(ctx, campaign.ID, source, voiceKey)
	if err != nil || cached == nil {
		respondEphemeral(s, i, fmt.Sprintf("No %s audio has been generated yet. Use `/campaign generate-recap-audio --source %s` first.", source, source))
		return
	}

	if _, err := os.Stat(cached.AudioPath); os.IsNotExist(err) {
		respondEphemeral(s, i, "Cached audio file is missing. Please regenerate with `/campaign generate-recap-audio`.")
		return
	}

	// Resolve the user's voice channel.
	guild, err := s.State.Guild(i.GuildID)
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
		respondEphemeral(s, i, "You must be in a voice channel to play the recap.")
		return
	}

	respond(s, i, fmt.Sprintf("Playing %s...", source))

	// Use existing voice connection if available, otherwise join temporarily.
	b.mu.Lock()
	existingVC := b.activeVC
	b.mu.Unlock()

	var vc *discordgo.VoiceConnection
	var shouldDisconnect bool

	if existingVC != nil {
		vc = existingVC
	} else {
		joinCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		vc, err = s.ChannelVoiceJoin(joinCtx, i.GuildID, userVoiceChannelID, false, true)
		cancel()
		if err != nil {
			log.Printf("play-recap: VoiceJoin error: %v", err)
			return
		}
		shouldDisconnect = true
	}

	log.Printf("play-recap: playing %s in channel %s", cached.AudioPath, userVoiceChannelID)
	if err := voice.PlayWAV(vc, cached.AudioPath); err != nil {
		log.Printf("play-recap: playback error: %v", err)
	}

	if shouldDisconnect {
		disconnCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		vc.Disconnect(disconnCtx)
		cancel()
	}
}

// resolveVoiceOption parses the "voice" autocomplete string option.
// Values are either a Discord user ID or "profile:{id}" for custom profiles.
func (b *Bot) resolveVoiceOption(opts map[string]*discordgo.ApplicationCommandInteractionDataOption) (userID string, profileID int64) {
	opt, ok := opts["voice"]
	if !ok {
		return "", 0
	}
	val := opt.StringValue()
	if val == "" {
		return "", 0
	}
	if strings.HasPrefix(val, "profile:") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(val, "profile:"), 10, 64)
		return "", id
	}
	return val, 0
}

// handleRecapVoiceAutocomplete provides autocomplete for the voice option,
// listing character names (mapped to user IDs) and uploaded voice profiles.
func (b *Bot) handleRecapVoiceAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		return
	}

	data := i.ApplicationCommandData()
	var input string
	if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
		for _, opt := range data.Options[0].Options {
			if opt.Name == "voice" && opt.Focused {
				input = strings.ToLower(opt.StringValue())
			}
		}
	}

	var choices []*discordgo.ApplicationCommandOptionChoice

	// Add campaign members with audio, showing character names.
	userIDs, _ := b.store.GetUsersWithAudio(ctx, campaign.ID)
	charMappings, _ := b.store.GetCharacterMappings(ctx, campaign.ID)
	charMap := make(map[string]string, len(charMappings))
	for _, m := range charMappings {
		charMap[m.UserID] = m.CharacterName
	}

	for _, uid := range userIDs {
		name := charMap[uid]
		if name == "" {
			name = b.ResolveUsername(uid)
		}
		if input == "" || strings.Contains(strings.ToLower(name), input) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  name,
				Value: uid,
			})
		}
		if len(choices) >= 25 {
			break
		}
	}

	// Add custom voice profiles.
	profiles, _ := b.store.GetVoiceProfiles(ctx, campaign.ID)
	for _, p := range profiles {
		if len(choices) >= 25 {
			break
		}
		label := p.Name + " (custom)"
		if input == "" || strings.Contains(strings.ToLower(p.Name), input) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  label,
				Value: fmt.Sprintf("profile:%d", p.ID),
			})
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}

// extractProfileRef loads a voice profile's audio as a reference clip.
func extractProfileRef(store *storage.Store, ctx context.Context, profileID int64) (*tts.ReferenceClip, error) {
	profile, err := store.GetVoiceProfile(ctx, profileID)
	if err != nil || profile == nil {
		return nil, fmt.Errorf("voice profile %d not found", profileID)
	}

	samples, err := audio.LoadRaw48k(profile.AudioPath)
	if err != nil {
		return nil, fmt.Errorf("load profile audio: %w", err)
	}

	// Limit to first 10 seconds.
	maxSamples := 10 * 48000
	if len(samples) > maxSamples {
		samples = samples[:maxSamples]
	}

	return &tts.ReferenceClip{
		Samples:    samples,
		SampleRate: 48000,
		Text:       profile.Transcript,
	}, nil
}

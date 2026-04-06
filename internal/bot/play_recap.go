package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"discord-rpg-summariser/internal/tts"
	"discord-rpg-summariser/internal/voice"

	"github.com/bwmarrin/discordgo"
)

// handleCampaignGenerateRecapAudio generates a TTS WAV file for the campaign
// recap and caches it for later playback.
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
	if campaign.Recap == "" {
		respondEphemeral(s, i, "No recap has been generated yet. Use `/campaign recap` first.")
		return
	}

	opts := subcommandOptions(i)
	voiceUserID := interactionUserID(i)
	if opt, ok := opts["voice"]; ok {
		voiceUserID = opt.UserValue().ID
	}

	// Acknowledge — generation takes a while.
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	ref, err := tts.ExtractReference(b.store, campaign.ID, voiceUserID)
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to extract reference audio for <@%s>: %v", voiceUserID, err),
		})
		return
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: "Generating recap audio... this may take a few minutes.",
	})

	samples, sampleRate, err := b.ttsSynth.Synthesize(campaign.Recap, ref.Samples, ref.SampleRate, ref.Text)
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("TTS generation failed: %v", err),
		})
		return
	}

	cacheDir := filepath.Join(b.config.Storage.AudioDir, "tts-cache")
	os.MkdirAll(cacheDir, 0o755)
	wavPath := filepath.Join(cacheDir, fmt.Sprintf("recap-%d-%s.wav", campaign.ID, voiceUserID))

	if err := tts.WriteWAV(wavPath, samples, sampleRate); err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to save audio: %v", err),
		})
		return
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: "Recap audio generated! Use `/campaign play-recap` to play it in a voice channel.",
	})
}

// handleCampaignPlayRecap plays a previously generated TTS recap in the user's
// voice channel. Does NOT generate — use generate-recap-audio first.
func (b *Bot) handleCampaignPlayRecap(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		return
	}

	opts := subcommandOptions(i)
	voiceUserID := interactionUserID(i)
	if opt, ok := opts["voice"]; ok {
		voiceUserID = opt.UserValue().ID
	}

	// Check for cached audio.
	cacheDir := filepath.Join(b.config.Storage.AudioDir, "tts-cache")
	wavPath := filepath.Join(cacheDir, fmt.Sprintf("recap-%d-%s.wav", campaign.ID, voiceUserID))

	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		respondEphemeral(s, i, "No recap audio has been generated yet. Use `/campaign generate-recap-audio` first.")
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

	respond(s, i, "Playing recap...")

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

	log.Printf("play-recap: playing %s in channel %s", wavPath, userVoiceChannelID)
	if err := voice.PlayWAV(vc, wavPath); err != nil {
		log.Printf("play-recap: playback error: %v", err)
	}

	if shouldDisconnect {
		disconnCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		vc.Disconnect(disconnCtx)
		cancel()
	}
}

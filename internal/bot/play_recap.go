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

func (b *Bot) handleCampaignPlayRecap(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	// Determine whose voice to use.
	opts := subcommandOptions(i)
	voiceUserID := interactionUserID(i)
	if opt, ok := opts["voice"]; ok {
		voiceUserID = opt.UserValue().ID
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

	// Check if we have a cached TTS file, otherwise generate.
	cacheDir := filepath.Join(b.config.Storage.AudioDir, "tts-cache")
	wavPath := filepath.Join(cacheDir, fmt.Sprintf("recap-%d-%s.wav", campaign.ID, voiceUserID))

	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		// Need to generate — acknowledge with a deferred response.
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

		os.MkdirAll(cacheDir, 0o755)
		if err := tts.WriteWAV(wavPath, samples, sampleRate); err != nil {
			s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("Failed to save audio: %v", err),
			})
			return
		}
	} else {
		// File exists — quick acknowledge.
		respond(s, i, "Playing recap...")
	}

	// Determine if we're already in a voice connection (recording session).
	b.mu.Lock()
	existingVC := b.activeVC
	b.mu.Unlock()

	var vc *discordgo.VoiceConnection
	var shouldDisconnect bool

	if existingVC != nil {
		// Already in a call — play into the existing connection.
		vc = existingVC
	} else {
		// Join the user's voice channel for playback.
		joinCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		vc, err = s.ChannelVoiceJoin(joinCtx, i.GuildID, userVoiceChannelID, false, true)
		cancel()
		if err != nil {
			log.Printf("play-recap: VoiceJoin error: %v", err)
			s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: "Failed to join voice channel.",
			})
			return
		}
		shouldDisconnect = true
	}

	// Play the audio.
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

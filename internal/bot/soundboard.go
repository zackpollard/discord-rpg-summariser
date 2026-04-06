package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	"discord-rpg-summariser/internal/voice"

	"github.com/bwmarrin/discordgo"
)

// PlayClipInVoice plays a WAV file through the bot's active voice connection.
// Implements the api.SoundboardPlayer interface.
func (b *Bot) PlayClipInVoice(wavPath string) error {
	b.mu.Lock()
	vc := b.activeVC
	b.mu.Unlock()

	if vc == nil {
		return fmt.Errorf("not in a voice channel")
	}

	return voice.PlayWAV(vc, wavPath)
}

func (b *Bot) handleSoundboardPlay(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		return
	}

	opts := subcommandOptions(i)
	clipName := ""
	if opt, ok := opts["clip"]; ok {
		clipName = opt.StringValue()
	}
	if clipName == "" {
		respondEphemeral(s, i, "Please specify a clip name.")
		return
	}

	clips, err := b.store.ListSoundboardClips(ctx, campaign.ID)
	if err != nil {
		respondEphemeral(s, i, "Failed to list clips.")
		return
	}

	// Find the clip by name (case-insensitive).
	var matchedClip *struct{ name, path string }
	for _, c := range clips {
		if strings.EqualFold(c.Name, clipName) {
			matchedClip = &struct{ name, path string }{c.Name, c.AudioPath}
			break
		}
	}
	if matchedClip == nil {
		respondEphemeral(s, i, fmt.Sprintf("Clip %q not found.", clipName))
		return
	}

	b.mu.Lock()
	vc := b.activeVC
	b.mu.Unlock()

	if vc == nil {
		respondEphemeral(s, i, "Bot is not in a voice channel. Start a session first.")
		return
	}

	respond(s, i, fmt.Sprintf("Playing %q...", matchedClip.name))

	go func() {
		if err := voice.PlayWAV(vc, matchedClip.path); err != nil {
			log.Printf("soundboard play: %v", err)
		}
	}()
}

func (b *Bot) handleSoundboardList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to resolve active campaign.")
		return
	}

	clips, err := b.store.ListSoundboardClips(ctx, campaign.ID)
	if err != nil {
		respondEphemeral(s, i, "Failed to list clips.")
		return
	}

	if len(clips) == 0 {
		respond(s, i, "No soundboard clips yet. Create clips from the session transcript on the web UI.")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Soundboard — %s** (%d clips)\n\n", campaign.Name, len(clips)))
	for _, c := range clips {
		dur := c.EndTime - c.StartTime
		sb.WriteString(fmt.Sprintf("• **%s** (%.1fs)\n", c.Name, dur))
	}

	respond(s, i, sb.String())
}

// handleSoundboardAutocomplete provides autocomplete suggestions for clip names.
func (b *Bot) handleSoundboardAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	campaign, err := b.store.GetOrCreateActiveCampaign(ctx, i.GuildID)
	if err != nil {
		return
	}

	clips, err := b.store.ListSoundboardClips(ctx, campaign.ID)
	if err != nil {
		return
	}

	// Get the current input value.
	data := i.ApplicationCommandData()
	var input string
	if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
		input = strings.ToLower(data.Options[0].Options[0].StringValue())
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	for _, c := range clips {
		if input == "" || strings.Contains(strings.ToLower(c.Name), input) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  c.Name,
				Value: c.Name,
			})
			if len(choices) >= 25 { // Discord limit
				break
			}
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}

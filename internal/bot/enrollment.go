package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"discord-rpg-summariser/internal/audio"
	"discord-rpg-summariser/internal/diarize"
	"discord-rpg-summariser/internal/voice"

	"github.com/bwmarrin/discordgo"
)

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

	// Defer the response -- enrollment takes ~10 seconds.
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

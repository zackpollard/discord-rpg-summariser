package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/telegram"
	"discord-rpg-summariser/internal/transcribe"
)

// ReprocessSession re-runs the summarisation and extraction pipeline on an
// existing session. If retranscribe is true, it also re-transcribes from the
// original WAV files (replacing existing transcript segments).
func (b *Bot) ReprocessSession(ctx context.Context, sessionID int64, retranscribe bool) error {
	session, err := b.store.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	b.store.UpdateSessionStatus(ctx, sessionID, "summarising")

	if retranscribe {
		if err := b.retranscribeSession(ctx, session); err != nil {
			log.Printf("reprocess: retranscription failed for session %d: %v", sessionID, err)
			b.store.UpdateSessionStatus(ctx, sessionID, "failed")
			return err
		}
	}

	// Load transcript segments from DB and format them.
	segments, err := b.store.GetTranscript(ctx, sessionID)
	if err != nil {
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		return fmt.Errorf("get transcript: %w", err)
	}
	if len(segments) == 0 {
		b.store.UpdateSessionSummary(ctx, sessionID, "No transcript data available.", nil)
		return nil
	}

	// Resolve character names for formatting.
	charNames := make(map[string]string)
	for _, seg := range segments {
		if _, ok := charNames[seg.UserID]; ok {
			continue
		}
		name, _ := b.store.GetCharacterName(ctx, seg.UserID, session.CampaignID)
		if name != "" {
			charNames[seg.UserID] = name
		}
	}

	// Build formatted transcript text.
	var merged []transcribe.UserSegment
	for _, seg := range segments {
		merged = append(merged, transcribe.UserSegment{
			UserID:        seg.UserID,
			CharacterName: charNames[seg.UserID],
			Segment: transcribe.Segment{
				StartTime: seg.StartTime,
				EndTime:   seg.EndTime,
				Text:      seg.Text,
			},
		})
	}

	// Resolve DM name.
	dmName := ""
	campaign, _ := b.store.GetCampaign(ctx, session.CampaignID)
	if campaign != nil && campaign.DMUserID != nil {
		if cn, _ := b.store.GetCharacterName(ctx, *campaign.DMUserID, campaign.ID); cn != "" {
			dmName = cn
		} else {
			dmName = b.ResolveUsername(*campaign.DMUserID)
		}
	}

	// Include stored Telegram messages in the transcript.
	transcript := b.buildTranscriptFromDB(ctx, session, campaign, merged, dmName)

	// Summarise.
	result, err := b.summariser.Summarise(ctx, transcript, "", dmName)
	if err != nil {
		log.Printf("reprocess: summarise failed for session %d: %v", sessionID, err)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		return err
	}

	if err := b.store.UpdateSessionSummary(ctx, sessionID, result.Summary, result.KeyEvents); err != nil {
		log.Printf("reprocess: UpdateSessionSummary: %v", err)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		return err
	}

	// Clean up old entity references before re-extracting.
	if err := b.store.DeleteEntityReferencesForSession(ctx, sessionID); err != nil {
		log.Printf("reprocess: DeleteEntityReferencesForSession: %v", err)
	}

	// Clean up old combat encounters before re-extracting.
	if err := b.store.DeleteCombatForSession(ctx, sessionID); err != nil {
		log.Printf("reprocess: DeleteCombatForSession: %v", err)
	}

	// Extract entities, quests, and combat (non-fatal).
	b.extractEntities(ctx, session, sessionID, transcript, result.Summary, dmName)
	b.extractQuests(ctx, session, sessionID, transcript, result.Summary, dmName)
	b.extractCombat(ctx, session, sessionID, transcript, result.Summary, dmName)

	// Regenerate embeddings: delete old ones first, then generate fresh.
	if err := b.store.DeleteEmbeddingsForSession(ctx, sessionID); err != nil {
		log.Printf("reprocess: DeleteEmbeddingsForSession: %v", err)
	}
	b.generateEmbeddings(ctx, session, sessionID, merged, result.Summary, dmName)

	log.Printf("reprocess: session %d completed successfully", sessionID)
	return nil
}

// buildTranscriptFromDB builds a formatted transcript from DB data, including
// any stored Telegram messages.
func (b *Bot) buildTranscriptFromDB(
	ctx context.Context,
	session *storage.Session,
	campaign *storage.Campaign,
	merged []transcribe.UserSegment,
	dmName string,
) string {
	// Load stored Telegram messages for this session.
	tgMsgs, err := b.store.GetTelegramMessages(ctx, session.ID, false)
	if err != nil {
		log.Printf("reprocess: GetTelegramMessages: %v", err)
		return transcribe.FormatTranscript(merged)
	}
	if len(tgMsgs) == 0 {
		return transcribe.FormatTranscript(merged)
	}

	var telegramDMID int64
	if campaign != nil && campaign.TelegramDMUserID != nil {
		telegramDMID = *campaign.TelegramDMUserID
	}

	senderLabel := "DM"
	if dmName != "" {
		senderLabel = dmName
	}

	var entries []transcribe.TelegramEntry
	for _, m := range tgMsgs {
		isDM := telegramDMID != 0 && m.FromUserID == telegramDMID
		if !telegram.IsRelevant(telegram.Message{
			FromID: m.FromUserID,
			Text:   m.Text,
		}, isDM) {
			continue
		}
		elapsed := m.SentAt.Sub(session.StartedAt).Seconds()
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

	log.Printf("reprocess: interleaving %d Telegram messages into transcript", len(entries))
	return transcribe.FormatTranscriptWithTelegram(merged, entries)
}

// retranscribeSession re-transcribes all WAV files in the session's audio
// directory, replacing existing transcript segments.
func (b *Bot) retranscribeSession(ctx context.Context, session *storage.Session) error {
	if session.AudioDir == "" {
		return fmt.Errorf("no audio directory for session %d", session.ID)
	}

	entries, err := os.ReadDir(session.AudioDir)
	if err != nil {
		return fmt.Errorf("read audio dir: %w", err)
	}

	// Find WAV files: each is named <user_id>.wav
	userFiles := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wav") {
			continue
		}
		userID := strings.TrimSuffix(entry.Name(), ".wav")
		userFiles[userID] = filepath.Join(session.AudioDir, entry.Name())
	}

	if len(userFiles) == 0 {
		return fmt.Errorf("no WAV files found in %s", session.AudioDir)
	}

	b.store.UpdateSessionStatus(ctx, session.ID, "transcribing")

	// Load shared mic config for this campaign.
	sharedMics, _ := b.store.GetSharedMics(ctx, session.CampaignID)
	sharedMicMap := make(map[string]storage.SharedMic, len(sharedMics))
	for _, m := range sharedMics {
		sharedMicMap[m.DiscordUserID] = m
	}

	userSegments := make(map[string][]transcribe.Segment, len(userFiles))
	for userID, wavPath := range userFiles {
		if mic, ok := sharedMicMap[userID]; ok {
			b.transcribeSharedMic(ctx, wavPath, mic, userSegments)
		} else {
			segs, err := b.transcriber.TranscribeFile(ctx, wavPath)
			if err != nil {
				log.Printf("reprocess: transcribe user %s: %v", userID, err)
				continue
			}
			userSegments[userID] = segs
		}
	}

	if len(userSegments) == 0 {
		return fmt.Errorf("all transcriptions failed")
	}

	// Resolve character names.
	charNames := make(map[string]string, len(userSegments))
	for userID := range userSegments {
		name, _ := b.store.GetCharacterName(ctx, userID, session.CampaignID)
		if name != "" {
			charNames[userID] = name
		}
	}

	// Reprocessed sessions don't have join offsets — use nil (zero offsets).
	merged := transcribe.MergeTranscripts(userSegments, charNames, nil)

	// Replace existing segments.
	if err := b.store.DeleteTranscriptSegments(ctx, session.ID); err != nil {
		return fmt.Errorf("delete old segments: %w", err)
	}

	var dbSegments []storage.TranscriptSegment
	for _, seg := range merged {
		dbSegments = append(dbSegments, storage.TranscriptSegment{
			SessionID: session.ID,
			UserID:    seg.UserID,
			StartTime: seg.StartTime,
			EndTime:   seg.EndTime,
			Text:      seg.Text,
		})
	}
	if err := b.store.InsertSegments(ctx, dbSegments); err != nil {
		return fmt.Errorf("insert new segments: %w", err)
	}

	return nil
}

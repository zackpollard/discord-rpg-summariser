package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"discord-rpg-summariser/internal/audio"
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/telegram"
	"discord-rpg-summariser/internal/transcribe"
)

// ReprocessSession re-runs the summarisation and extraction pipeline on an
// existing session. If retranscribe is true, it also re-transcribes from the
// original WAV files (replacing existing transcript segments).
func (b *Bot) ReprocessSession(ctx context.Context, sessionID int64, retranscribe bool) error {
	ctx = summarise.WithSessionID(ctx, sessionID)

	// Set up progress tracking.
	b.mu.Lock()
	b.progress = NewPipelineProgress(sessionID)
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.progress = nil
		b.mu.Unlock()
	}()

	// Stream LLM stderr to the progress window.
	if cli, ok := b.summariser.(*summarise.ClaudeCLI); ok {
		progress := b.progress
		cli.OnStream = func(operation, message string) {
			progress.BroadcastLog(fmt.Sprintf("[%s] %s", operation, message))
		}
		defer func() { cli.OnStream = nil }()
	}

	session, err := b.store.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	if retranscribe {
		b.store.UpdateSessionStatus(ctx, sessionID, "transcribing")
		b.progress.SetStage("transcribing", "Re-transcribing audio")
		if err := b.retranscribeSession(ctx, session); err != nil {
			log.Printf("reprocess: retranscription failed for session %d: %v", sessionID, err)
			b.store.UpdateSessionStatus(ctx, sessionID, "failed")
			return err
		}
	} else {
		// Skip the transcription weight so the progress bar starts at the
		// right place instead of jumping from 0% to 60%.
		b.progress.SkipStage("transcribing")
		b.progress.SkipStage("mixing")
	}

	b.store.UpdateSessionStatus(ctx, sessionID, "summarising")

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
	campaign, _ := b.store.GetCampaign(ctx, session.CampaignID)
	for _, seg := range segments {
		if _, ok := charNames[seg.UserID]; ok {
			continue
		}
		// Label the DM.
		if campaign != nil && campaign.DMUserID != nil && seg.UserID == *campaign.DMUserID {
			charNames[seg.UserID] = "DM"
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
	if campaign != nil && campaign.DMUserID != nil {
		if cn, _ := b.store.GetCharacterName(ctx, *campaign.DMUserID, campaign.ID); cn != "" {
			dmName = cn
		} else {
			dmName = b.ResolveUsername(*campaign.DMUserID)
		}
	}

	// Annotate transcript: classify segments, correct ASR errors, detect
	// scene boundaries, and identify NPC voices. Required — downstream
	// stages depend on the annotated transcript for quality.
	b.progress.SetStage("summarising", "Annotating transcript")
	annotations := b.annotateTranscript(ctx, session, sessionID, merged, charNames, dmName)

	if len(annotations) == 0 {
		log.Printf("reprocess: annotation failed for session %d, aborting", sessionID)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		return fmt.Errorf("transcript annotation failed")
	}

	transcript := buildAnnotatedTranscript(merged, annotations, dmName)

	// Summarise.
	b.progress.SetStage("summarising", "Generating summary")
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

	// Clean up old data before re-extracting.
	if err := b.store.DeleteEntityReferencesForSession(ctx, sessionID); err != nil {
		log.Printf("reprocess: DeleteEntityReferencesForSession: %v", err)
	}
	if err := b.store.DeleteCombatForSession(ctx, sessionID); err != nil {
		log.Printf("reprocess: DeleteCombatForSession: %v", err)
	}

	// Run extraction stages in parallel.
	b.progress.SetStage("extracting", "Extracting title, entities, quests, and combat")

	var extractWg sync.WaitGroup
	extractWg.Add(4)

	go func() {
		defer extractWg.Done()
		b.extractTitleAndQuotes(ctx, session, sessionID, transcript, result.Summary, dmName)
	}()

	go func() {
		defer extractWg.Done()
		b.extractEntities(ctx, session, sessionID, transcript, result.Summary, dmName)
	}()

	go func() {
		defer extractWg.Done()
		b.extractQuests(ctx, session, sessionID, transcript, result.Summary, dmName)
	}()

	go func() {
		defer extractWg.Done()
		b.extractCombat(ctx, session, sessionID, transcript, result.Summary, dmName)
		b.extractCreatures(ctx, session, sessionID, transcript, result.Summary, dmName)
	}()

	extractWg.Wait()

	// Regenerate embeddings after extractions complete.
	b.progress.SetStage("generating embeddings", "Generating embeddings")
	if err := b.store.DeleteEmbeddingsForSession(ctx, sessionID); err != nil {
		log.Printf("reprocess: DeleteEmbeddingsForSession: %v", err)
	}
	b.generateEmbeddings(ctx, session, sessionID, merged, result.Summary, dmName)

	b.progress.Complete()
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
		if userID == "mixed" {
			continue // skip the cached mixed-down file
		}
		userFiles[userID] = filepath.Join(session.AudioDir, entry.Name())
	}

	if len(userFiles) == 0 {
		return fmt.Errorf("no WAV files found in %s", session.AudioDir)
	}

	b.store.UpdateSessionStatus(ctx, session.ID, "transcribing")

	transcriber, err := b.acquireTranscriber()
	if err != nil {
		return fmt.Errorf("load transcriber: %w", err)
	}
	defer b.releaseTranscriber()

	if campaign, _ := b.store.GetCampaign(ctx, session.CampaignID); campaign != nil {
		transcriber.SetGameSystem(campaign.GameSystem)
	}
	transcriber.SetVocabulary(b.gatherCampaignVocabulary(ctx, session.CampaignID))

	// Load shared mic config for this campaign.
	sharedMics, _ := b.store.GetSharedMics(ctx, session.CampaignID)
	sharedMicMap := make(map[string]storage.SharedMic, len(sharedMics))
	for _, m := range sharedMics {
		sharedMicMap[m.DiscordUserID] = m
	}

	totalUsers := len(userFiles)

	// Wire up intra-file progress if the transcriber supports it.
	type progressSetter interface {
		SetProgressCallback(func(float64))
	}
	setIntraProgress := func(doneUsers int) {
		if ps, ok := transcriber.(progressSetter); ok {
			ps.SetProgressCallback(func(filePct float64) {
				p := (float64(doneUsers) + filePct) / float64(totalUsers)
				b.progress.SetSubProgress(p)
			})
		}
	}

	userSegments := make(map[string][]transcribe.Segment, len(userFiles))
	doneUsers := 0
	for userID, wavPath := range userFiles {
		setIntraProgress(doneUsers)
		if mic, ok := sharedMicMap[userID]; ok {
			b.transcribeSharedMic(ctx, transcriber, wavPath, mic, userSegments)
		} else {
			segs, err := transcriber.TranscribeFile(ctx, wavPath)
			if err != nil {
				log.Printf("reprocess: transcribe user %s: %v", userID, err)
				doneUsers++
				b.progress.SetSubProgress(float64(doneUsers) / float64(totalUsers))
				continue
			}
			userSegments[userID] = segs
		}
		doneUsers++
		b.progress.SetDetail(fmt.Sprintf("Re-transcribing audio (%d of %d users)", doneUsers, totalUsers))
		b.progress.SetSubProgress(float64(doneUsers) / float64(totalUsers))
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

	// Load persisted join offsets if available (nil for older sessions).
	joinOffsets := audio.LoadJoinOffsets(session.AudioDir)
	merged := transcribe.MergeTranscripts(userSegments, charNames, joinOffsets)

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

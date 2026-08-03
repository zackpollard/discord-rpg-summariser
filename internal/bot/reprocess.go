package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"discord-rpg-summariser/internal/audio"
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/transcribe"
)

// ReprocessSession re-runs the summarisation and extraction pipeline on an
// existing session. If retranscribe is true, it also re-transcribes from the
// original WAV files (replacing existing transcript segments).
func (b *Bot) ReprocessSession(ctx context.Context, sessionID int64, retranscribe bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("reprocess: panic for session %d: %v\n%s", sessionID, r, debug.Stack())
			b.store.UpdateSessionStatus(context.Background(), sessionID, "failed")
			err = fmt.Errorf("reprocess panicked: %v", r)
		}
	}()

	ctx = summarise.WithSessionID(ctx, sessionID)

	// Set up progress tracking. The tracker is kept in a local for the whole
	// run; registering it also claims the session so two runs can't interleave
	// their DB writes.
	prog, ok := b.beginPipeline(sessionID)
	if !ok {
		return fmt.Errorf("a pipeline run for session %d is already in progress", sessionID)
	}
	defer b.endPipeline(sessionID, prog)

	// Stream LLM stderr to the progress window.
	if cli, isCLI := b.summariser.(*summarise.ClaudeCLI); isCLI {
		cli.SetOnStream(func(operation, message string) {
			prog.BroadcastLog(fmt.Sprintf("[%s] %s", operation, message))
		})
		defer cli.SetOnStream(nil)
	}

	session, err := b.store.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	if retranscribe {
		b.store.UpdateSessionStatus(ctx, sessionID, "transcribing")
		prog.SetStage("transcribing", "Re-transcribing audio")
		if err := b.retranscribeSession(ctx, prog, session); err != nil {
			log.Printf("reprocess: retranscription failed for session %d: %v", sessionID, err)
			b.store.UpdateSessionStatus(ctx, sessionID, "failed")
			return err
		}
	} else {
		// Skip the transcription weight so the progress bar starts at the
		// right place instead of jumping from 0% to 60%.
		prog.SkipStage("transcribing")
		prog.SkipStage("mixing")
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

	// Build formatted transcript text, keeping each segment's DB ID alongside
	// it so annotations can be matched by identity rather than by position.
	var merged []transcribe.UserSegment
	segmentIDs := make([]int64, 0, len(segments))
	for _, seg := range segments {
		segmentIDs = append(segmentIDs, seg.ID)
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
	prog.SetStage("summarising", "Annotating transcript")
	annotations, annotatedIDs := b.annotateTranscript(ctx, prog, session, sessionID, merged, charNames, dmName)

	if len(annotations) == 0 {
		log.Printf("reprocess: annotation failed for session %d, aborting", sessionID)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		return fmt.Errorf("transcript annotation failed")
	}
	if len(annotatedIDs) > 0 {
		segmentIDs = annotatedIDs
	}

	transcript := buildAnnotatedTranscript(merged, segmentIDs, annotations, dmName)

	// Interleave any Telegram messages captured during the session.
	transcript = b.interleaveTelegramIntoAnnotated(ctx, session, campaign, transcript, dmName)

	// Summarise.
	prog.SetStage("summarising", "Generating summary")
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
	prog.SetStage("extracting", "Extracting title, entities, quests, and combat")

	var extractWg sync.WaitGroup
	extractWg.Add(4)

	go func() {
		defer extractWg.Done()
		defer recoverPanic("reprocess title/quotes extraction")
		b.extractTitleAndQuotes(ctx, session, sessionID, transcript, result.Summary, dmName)
	}()

	go func() {
		defer extractWg.Done()
		defer recoverPanic("reprocess entity extraction")
		b.extractEntities(ctx, session, sessionID, transcript, result.Summary, dmName)
	}()

	go func() {
		defer extractWg.Done()
		defer recoverPanic("reprocess quest extraction")
		b.extractQuests(ctx, session, sessionID, transcript, result.Summary, dmName)
	}()

	go func() {
		defer extractWg.Done()
		defer recoverPanic("reprocess combat extraction")
		b.extractCombat(ctx, session, sessionID, transcript, result.Summary, dmName)
		b.extractCreatures(ctx, session, sessionID, transcript, result.Summary, dmName)
	}()

	extractWg.Wait()

	// Regenerate embeddings after extractions complete.
	prog.SetStage("generating embeddings", "Generating embeddings")
	if err := b.store.DeleteEmbeddingsForSession(ctx, sessionID); err != nil {
		log.Printf("reprocess: DeleteEmbeddingsForSession: %v", err)
	}
	b.generateEmbeddings(ctx, session, sessionID, merged, result.Summary, dmName)

	prog.Complete()
	log.Printf("reprocess: session %d completed successfully", sessionID)
	return nil
}

// retranscribeSession re-transcribes all WAV files in the session's audio
// directory, replacing existing transcript segments.
func (b *Bot) retranscribeSession(ctx context.Context, prog *PipelineProgress, session *storage.Session) error {
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
				prog.SetSubProgress(p)
			})
		}
	}

	userSegments := make(map[string][]transcribe.Segment, len(userFiles))
	doneUsers := 0
	overallStart := time.Now()
	log.Printf("reprocess: starting transcription of %d users for session %d", totalUsers, session.ID)
	for userID, wavPath := range userFiles {
		userStart := time.Now()
		log.Printf("reprocess: transcribing user %s (%d of %d)", userID, doneUsers+1, totalUsers)
		setIntraProgress(doneUsers)
		if mic, ok := sharedMicMap[userID]; ok {
			b.transcribeSharedMic(ctx, transcriber, wavPath, mic, userSegments)
		} else {
			segs, err := transcriber.TranscribeFile(ctx, wavPath)
			if err != nil {
				log.Printf("reprocess: transcribe user %s: %v", userID, err)
				doneUsers++
				prog.SetSubProgress(float64(doneUsers) / float64(totalUsers))
				continue
			}
			userSegments[userID] = segs
		}
		doneUsers++
		log.Printf("reprocess: user %s done in %s (%d of %d)", userID, time.Since(userStart).Round(time.Second), doneUsers, totalUsers)
		prog.SetDetail(fmt.Sprintf("Re-transcribing audio (%d of %d users)", doneUsers, totalUsers))
		prog.SetSubProgress(float64(doneUsers) / float64(totalUsers))
	}
	log.Printf("reprocess: all %d users transcribed in %s", totalUsers, time.Since(overallStart).Round(time.Second))

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

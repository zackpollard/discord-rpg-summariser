package bot

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"

	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/transcribe"
)

// RerunStages re-runs specific pipeline stages for an existing session.
// Valid stage names: annotate, summarise, title_quotes, entities, quests, combat, creatures, embeddings
func (b *Bot) RerunStages(ctx context.Context, sessionID int64, stages []string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("rerun: panic for session %d: %v\n%s", sessionID, r, debug.Stack())
			b.store.UpdateSessionStatus(context.Background(), sessionID, "failed")
			err = fmt.Errorf("rerun panicked: %v", r)
		}
	}()

	ctx = summarise.WithSessionID(ctx, sessionID)

	prog, ok := b.beginPipeline(sessionID)
	if !ok {
		return fmt.Errorf("a pipeline run for session %d is already in progress", sessionID)
	}
	defer b.endPipeline(sessionID, prog)

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

	// Load transcript and character names.
	segments, err := b.store.GetTranscript(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get transcript: %w", err)
	}

	campaign, _ := b.store.GetCampaign(ctx, session.CampaignID)

	charNames := make(map[string]string)
	for _, seg := range segments {
		if _, ok := charNames[seg.UserID]; ok {
			continue
		}
		if campaign != nil && campaign.DMUserID != nil && seg.UserID == *campaign.DMUserID {
			charNames[seg.UserID] = "DM"
			continue
		}
		name, _ := b.store.GetCharacterName(ctx, seg.UserID, session.CampaignID)
		if name != "" {
			charNames[seg.UserID] = name
		}
	}

	// Keep each segment's DB ID alongside it so annotations are matched by
	// identity rather than by position.
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

	dmName := ""
	if campaign != nil && campaign.DMUserID != nil {
		if cn, _ := b.store.GetCharacterName(ctx, *campaign.DMUserID, campaign.ID); cn != "" {
			dmName = cn
		} else {
			dmName = b.ResolveUsername(*campaign.DMUserID)
		}
	}

	// Build the transcript from existing annotations if available, or raw.
	annotations, _ := b.store.GetAnnotations(ctx, sessionID)
	var annotationMap map[int64]*storage.TranscriptAnnotation
	if len(annotations) > 0 {
		annotationMap = make(map[int64]*storage.TranscriptAnnotation, len(annotations))
		for i := range annotations {
			annotationMap[annotations[i].SegmentID] = &annotations[i]
		}
	}

	stageSet := make(map[string]bool, len(stages))
	for _, s := range stages {
		stageSet[s] = true
	}

	totalStages := len(stages)
	doneStages := 0
	updateProgress := func(name string) {
		doneStages++
		pct := float64(doneStages) / float64(totalStages) * 100
		prog.broadcast(ProgressEvent{
			Type:    "progress",
			Stage:   name,
			Detail:  fmt.Sprintf("Running %s (%d/%d)", name, doneStages, totalStages),
			Percent: pct,
			ETA:     -1,
		})
	}

	b.store.UpdateSessionStatus(ctx, sessionID, "summarising")

	// Run requested stages.
	if stageSet["annotate"] {
		prog.SetStageLabel("summarising", "Annotating transcript")
		newAnnotations, annotatedIDs := b.annotateTranscript(ctx, prog, session, sessionID, merged, charNames, dmName)
		if len(newAnnotations) > 0 {
			annotationMap = newAnnotations
			if len(annotatedIDs) > 0 {
				segmentIDs = annotatedIDs
			}
		}
		updateProgress("annotate")
	}

	// Build transcript for LLM stages.
	var transcript string
	if len(annotationMap) > 0 {
		transcript = buildAnnotatedTranscript(merged, segmentIDs, annotationMap, dmName)
	} else {
		transcript = transcribe.FormatTranscript(merged)
	}
	transcript = b.interleaveTelegramIntoAnnotated(ctx, session, campaign, transcript, dmName)

	// Get existing summary for stages that need it.
	summary := ""
	if session.Summary != nil {
		summary = *session.Summary
	}

	stageFailed := false
	if stageSet["summarise"] {
		prog.SetStageLabel("summarising", "Generating summary")
		result, err := b.summariser.Summarise(ctx, transcript, "", dmName)
		if err != nil {
			log.Printf("rerun: summarise failed: %v", err)
			stageFailed = true
		} else if err := b.store.UpdateSessionSummary(ctx, sessionID, result.Summary, result.KeyEvents); err != nil {
			log.Printf("rerun: UpdateSessionSummary: %v", err)
			stageFailed = true
		} else {
			summary = result.Summary
		}
		updateProgress("summarise")
	}

	if stageSet["title_quotes"] {
		prog.SetStageLabel("extracting", "Generating title and quotes")
		b.extractTitleAndQuotes(ctx, session, sessionID, transcript, summary, dmName)
		updateProgress("title_quotes")
	}

	if stageSet["entities"] {
		prog.SetStageLabel("extracting", "Extracting entities")
		b.store.DeleteEntityReferencesForSession(ctx, sessionID)
		b.extractEntities(ctx, session, sessionID, transcript, summary, dmName)
		updateProgress("entities")
	}

	if stageSet["quests"] {
		prog.SetStageLabel("extracting", "Extracting quests")
		b.extractQuests(ctx, session, sessionID, transcript, summary, dmName)
		updateProgress("quests")
	}

	if stageSet["combat"] {
		prog.SetStageLabel("extracting", "Extracting combat encounters")
		b.store.DeleteCombatForSession(ctx, sessionID)
		b.extractCombat(ctx, session, sessionID, transcript, summary, dmName)
		updateProgress("combat")
	}

	if stageSet["creatures"] {
		prog.SetStageLabel("extracting", "Identifying creatures for bestiary")
		b.extractCreatures(ctx, session, sessionID, transcript, summary, dmName)
		updateProgress("creatures")
	}

	if stageSet["embeddings"] {
		prog.SetStageLabel("generating embeddings", "Generating embeddings")
		b.store.DeleteEmbeddingsForSession(ctx, sessionID)
		b.generateEmbeddings(ctx, session, sessionID, merged, summary, dmName)
		updateProgress("embeddings")
	}

	// Don't advertise a terminal "complete" when the summarise stage errored —
	// the session would show as good data with a stale or missing summary.
	// A rerun is usually invoked on an already-complete session though, so
	// fall back to the status it came in with rather than demoting a row whose
	// stored summary is still perfectly good: "failed" would drop it out of
	// GetLatestCompleteSessions and hide it from recaps and exports.
	finalStatus := "complete"
	if stageFailed {
		finalStatus = session.Status
		if session.Summary == nil || *session.Summary == "" {
			finalStatus = "failed"
		}
	}
	b.store.UpdateSessionStatus(ctx, sessionID, finalStatus)
	// Complete() regardless: it is the only event that terminates the SSE
	// stream, and the client reloads the session to pick up the real status.
	prog.Complete()
	if stageFailed {
		log.Printf("rerun: session %d stages %v finished with failures", sessionID, stages)
		return fmt.Errorf("rerun: summarise stage failed for session %d", sessionID)
	}
	log.Printf("rerun: session %d stages %v completed", sessionID, stages)
	return nil
}

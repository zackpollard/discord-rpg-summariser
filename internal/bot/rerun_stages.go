package bot

import (
	"context"
	"fmt"
	"log"

	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/transcribe"
)

// RerunStages re-runs specific pipeline stages for an existing session.
// Valid stage names: annotate, summarise, title_quotes, entities, quests, combat, embeddings
func (b *Bot) RerunStages(ctx context.Context, sessionID int64, stages []string) error {
	ctx = summarise.WithSessionID(ctx, sessionID)

	b.mu.Lock()
	b.progress = NewPipelineProgress(sessionID)
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.progress = nil
		b.mu.Unlock()
	}()

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
		b.progress.broadcast(ProgressEvent{
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
		b.progress.SetStage("summarising", "Annotating transcript")
		newAnnotations := b.annotateTranscript(ctx, session, sessionID, merged, charNames, dmName)
		if len(newAnnotations) > 0 {
			annotationMap = newAnnotations
		}
		updateProgress("annotate")
	}

	// Build transcript for LLM stages.
	var transcript string
	if len(annotationMap) > 0 {
		transcript = buildAnnotatedTranscript(merged, annotationMap, dmName)
	} else {
		transcript = transcribe.FormatTranscript(merged)
	}

	// Get existing summary for stages that need it.
	summary := ""
	if session.Summary != nil {
		summary = *session.Summary
	}

	if stageSet["summarise"] {
		b.progress.SetStage("summarising", "Generating summary")
		result, err := b.summariser.Summarise(ctx, transcript, "", dmName)
		if err != nil {
			log.Printf("rerun: summarise failed: %v", err)
		} else {
			b.store.UpdateSessionSummary(ctx, sessionID, result.Summary, result.KeyEvents)
			summary = result.Summary
		}
		updateProgress("summarise")
	}

	if stageSet["title_quotes"] {
		b.progress.SetStage("extracting title", "Generating title and quotes")
		b.extractTitleAndQuotes(ctx, session, sessionID, transcript, summary, dmName)
		updateProgress("title_quotes")
	}

	if stageSet["entities"] {
		b.progress.SetStage("extracting entities", "Extracting entities")
		b.store.DeleteEntityReferencesForSession(ctx, sessionID)
		b.extractEntities(ctx, session, sessionID, transcript, summary, dmName)
		updateProgress("entities")
	}

	if stageSet["quests"] {
		b.progress.SetStage("extracting quests", "Extracting quests")
		b.extractQuests(ctx, session, sessionID, transcript, summary, dmName)
		updateProgress("quests")
	}

	if stageSet["combat"] {
		b.progress.SetStage("extracting combat", "Extracting combat encounters")
		b.store.DeleteCombatForSession(ctx, sessionID)
		b.extractCombat(ctx, session, sessionID, transcript, summary, dmName)
		updateProgress("combat")
	}

	if stageSet["embeddings"] {
		b.progress.SetStage("generating embeddings", "Generating embeddings")
		b.store.DeleteEmbeddingsForSession(ctx, sessionID)
		b.generateEmbeddings(ctx, session, sessionID, merged, summary, dmName)
		updateProgress("embeddings")
	}

	b.store.UpdateSessionStatus(ctx, sessionID, "complete")
	b.progress.Complete()
	log.Printf("rerun: session %d stages %v completed", sessionID, stages)
	return nil
}

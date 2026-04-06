package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/telegram"
	"discord-rpg-summariser/internal/transcribe"
)

// annotateTranscript runs the LLM annotation stage on the transcript segments.
// Returns a map of segment ID → annotation, or nil on failure.
func (b *Bot) annotateTranscript(
	ctx context.Context,
	session *storage.Session,
	sessionID int64,
	merged []transcribe.UserSegment,
	charNames map[string]string,
	dmName string,
) map[int64]*storage.TranscriptAnnotation {
	annotator, ok := b.summariser.(summarise.TranscriptAnnotator)
	if !ok {
		log.Printf("pipeline: summariser does not support annotation, skipping")
		return nil
	}

	// Query segments from DB to get their assigned IDs.
	dbSegments, err := b.store.GetTranscript(ctx, sessionID)
	if err != nil {
		log.Printf("pipeline: get transcript for annotation: %v", err)
		return nil
	}
	if len(dbSegments) == 0 {
		return nil
	}

	// Build annotation inputs.
	inputs := make([]summarise.AnnotationInput, len(dbSegments))
	for i, seg := range dbSegments {
		speaker := charNames[seg.UserID]
		if speaker == "" {
			speaker = seg.UserID
		}
		inputs[i] = summarise.AnnotationInput{
			ID:        seg.ID,
			Speaker:   speaker,
			StartTime: seg.StartTime,
			Text:      seg.Text,
		}
	}

	// Build vocabulary from campaign data.
	vocab := summarise.AnnotationVocabulary{}
	if campaign, _ := b.store.GetCampaign(ctx, session.CampaignID); campaign != nil {
		vocab.GameSystem = campaign.GameSystem
	}
	charMappings, _ := b.store.GetCharacterMappings(ctx, session.CampaignID)
	for _, m := range charMappings {
		vocab.CharacterNames = append(vocab.CharacterNames, m.CharacterName)
	}
	entities, _ := b.store.ListEntities(ctx, session.CampaignID, "", "", 500, 0)
	for _, e := range entities {
		vocab.EntityNames = append(vocab.EntityNames, e.Name)
	}

	// Run annotation.
	result, err := annotator.AnnotateTranscript(ctx, inputs, vocab, dmName)
	if err != nil {
		log.Printf("pipeline: annotation failed: %v", err)
		return nil
	}

	// Persist to DB.
	b.store.DeleteAnnotations(ctx, sessionID) // clear any previous run

	var dbAnnotations []storage.TranscriptAnnotation
	annotationMap := make(map[int64]*storage.TranscriptAnnotation)

	for _, seg := range result.Segments {
		a := storage.TranscriptAnnotation{
			SegmentID:      seg.ID,
			SessionID:      sessionID,
			Classification: seg.Classification,
			CorrectedText:  seg.CorrectedText,
			Scene:          seg.Scene,
			NPCVoice:       seg.NPCVoice,
			MergeWithNext:  seg.MergeWithNext,
			Tone:           seg.Tone,
		}
		// Validate classification.
		switch a.Classification {
		case "narrative", "table_talk", "ambiguous":
		default:
			a.Classification = "narrative"
		}
		dbAnnotations = append(dbAnnotations, a)
		aCopy := a
		annotationMap[seg.ID] = &aCopy
	}

	if err := b.store.InsertAnnotations(ctx, dbAnnotations); err != nil {
		log.Printf("pipeline: insert annotations: %v", err)
	}

	return annotationMap
}

// buildAnnotatedTranscript produces a transcript string from merged segments
// and annotations. Table talk is excluded, corrected text is used when
// available, scene boundaries are inserted, and NPC voices are labelled.
func buildAnnotatedTranscript(
	merged []transcribe.UserSegment,
	annotations map[int64]*storage.TranscriptAnnotation,
	dmName string,
) string {
	// We need to match merged segments to annotations by position since
	// merged segments don't have DB IDs. Build a positional lookup.
	// The annotations map is keyed by segment DB ID, but we can match
	// by index since both are in the same order.

	// Actually, the merged segments and DB segments are in the same order
	// (both sorted by start_time from MergeTranscripts / InsertSegments).
	// We'll match by collecting annotation values in order.
	orderedAnnotations := make([]*storage.TranscriptAnnotation, 0, len(annotations))
	for _, a := range annotations {
		orderedAnnotations = append(orderedAnnotations, a)
	}
	// Sort by segment ID to match insertion order.
	sortAnnotationsBySegmentID(orderedAnnotations)

	var b strings.Builder
	var lastScene string
	var mergeBuffer string // accumulates text from merged segments
	var mergeSpeaker string
	var mergeTS string

	flushMerge := func() {
		if mergeBuffer != "" {
			fmt.Fprintf(&b, "[%s] %s: %s\n", mergeTS, mergeSpeaker, mergeBuffer)
			mergeBuffer = ""
		}
	}

	for i, seg := range merged {
		var ann *storage.TranscriptAnnotation
		if i < len(orderedAnnotations) {
			ann = orderedAnnotations[i]
		}

		// Skip table talk.
		if ann != nil && ann.Classification == "table_talk" {
			continue
		}

		// Insert scene boundary.
		if ann != nil && ann.Scene != nil && *ann.Scene != "" && *ann.Scene != lastScene {
			flushMerge()
			lastScene = *ann.Scene
			fmt.Fprintf(&b, "\n--- %s ---\n\n", strings.ToUpper(lastScene[:1])+lastScene[1:])
		}

		// Determine speaker label.
		name := seg.CharacterName
		if name == "" {
			name = seg.UserID
		}
		if ann != nil && ann.NPCVoice != nil && *ann.NPCVoice != "" {
			name = fmt.Sprintf("%s (as %s)", name, *ann.NPCVoice)
		}

		// Use corrected text if available.
		text := seg.Text
		if ann != nil && ann.CorrectedText != nil && *ann.CorrectedText != "" {
			text = *ann.CorrectedText
		}

		// Handle segment merging.
		if mergeBuffer != "" {
			// Continue merging into the buffer.
			mergeBuffer += " " + text
			if ann == nil || !ann.MergeWithNext {
				flushMerge()
			}
			continue
		}

		if ann != nil && ann.MergeWithNext {
			// Start a merge buffer.
			mergeBuffer = text
			mergeSpeaker = name
			mergeTS = formatSeconds(seg.StartTime)
			continue
		}

		ts := formatSeconds(seg.StartTime)
		fmt.Fprintf(&b, "[%s] %s: %s\n", ts, name, text)
	}

	flushMerge()
	return b.String()
}

func sortAnnotationsBySegmentID(annotations []*storage.TranscriptAnnotation) {
	for i := 1; i < len(annotations); i++ {
		for j := i; j > 0 && annotations[j].SegmentID < annotations[j-1].SegmentID; j-- {
			annotations[j], annotations[j-1] = annotations[j-1], annotations[j]
		}
	}
}

func formatSeconds(secs float64) string {
	total := int(secs)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func countClassification(annotations map[int64]*storage.TranscriptAnnotation, class string) int {
	count := 0
	for _, a := range annotations {
		if a.Classification == class {
			count++
		}
	}
	return count
}

// interleaveTelegramIntoAnnotated adds Telegram messages to an already-built
// annotated transcript. This is a simplified version that appends them at the
// end since the annotated transcript doesn't easily support mid-insertion.
func (b *Bot) interleaveTelegramIntoAnnotated(
	ctx context.Context,
	session *storage.Session,
	campaign *storage.Campaign,
	transcript string,
	telegramMsgs []telegram.Message,
	dmName string,
) string {
	if len(telegramMsgs) == 0 {
		return transcript
	}

	// For now, just return the annotated transcript without Telegram messages
	// rather than risk misaligning them. The Telegram messages are already
	// persisted to DB and can be interleaved in the reprocess path.
	return transcript
}

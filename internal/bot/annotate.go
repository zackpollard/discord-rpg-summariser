package bot

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/telegram"
	"discord-rpg-summariser/internal/transcribe"
)

// annotateTranscript runs the LLM annotation stage on the transcript segments.
// Returns a map of segment ID → annotation and the DB segment IDs in
// transcript order (so callers can pair annotations with merged segments), or
// nil on failure.
func (b *Bot) annotateTranscript(
	ctx context.Context,
	prog *PipelineProgress,
	session *storage.Session,
	sessionID int64,
	merged []transcribe.UserSegment,
	charNames map[string]string,
	dmName string,
) (map[int64]*storage.TranscriptAnnotation, []int64) {
	annotator, ok := b.summariser.(summarise.TranscriptAnnotator)
	if !ok {
		log.Printf("pipeline: summariser does not support annotation, skipping")
		return nil, nil
	}

	// Query segments from DB to get their assigned IDs.
	dbSegments, err := b.store.GetTranscript(ctx, sessionID)
	if err != nil {
		log.Printf("pipeline: get transcript for annotation: %v", err)
		return nil, nil
	}
	if len(dbSegments) == 0 {
		return nil, nil
	}

	// Build annotation inputs.
	inputs := make([]summarise.AnnotationInput, len(dbSegments))
	segmentIDs := make([]int64, len(dbSegments))
	validIDs := make(map[int64]struct{}, len(dbSegments))
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
		segmentIDs[i] = seg.ID
		validIDs[seg.ID] = struct{}{}
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

	// Chunk the segments to avoid overwhelming the LLM. Use Claude's session
	// resume to maintain full context across batches — Claude keeps the entire
	// conversation history internally so it knows what was said in prior batches.
	const batchSize = 200
	var allAnnotated []summarise.AnnotatedSegment

	// Try the session-aware batch method first.
	batchAnnotator, hasBatch := annotator.(*summarise.ClaudeCLI)

	var claudeSessionID string
	for i := 0; i < len(inputs); i += batchSize {
		end := i + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		batch := inputs[i:end]

		log.Printf("pipeline: annotating batch %d-%d of %d segments (session: %s)",
			i+1, end, len(inputs), claudeSessionID)
		if prog != nil {
			prog.SetDetail(fmt.Sprintf("Annotating transcript (%d/%d segments)", end, len(inputs)))
		}

		var result *summarise.AnnotationResult
		var err error

		if hasBatch {
			var sid string
			result, sid, err = batchAnnotator.AnnotateTranscriptBatch(ctx, batch, vocab, dmName, claudeSessionID)
			if sid != "" {
				claudeSessionID = sid
			}
		} else {
			result, err = annotator.AnnotateTranscript(ctx, batch, vocab, dmName)
		}

		if err != nil {
			log.Printf("pipeline: annotation batch %d-%d failed: %v", i+1, end, err)
			continue
		}
		allAnnotated = append(allAnnotated, result.Segments...)
	}

	if len(allAnnotated) == 0 {
		log.Printf("pipeline: all annotation batches failed")
		return nil, nil
	}

	// Persist to DB.
	b.store.DeleteAnnotations(ctx, sessionID) // clear any previous run

	var dbAnnotations []storage.TranscriptAnnotation
	annotationMap := make(map[int64]*storage.TranscriptAnnotation)

	dropped := 0
	for _, seg := range allAnnotated {
		// The segment ID comes straight out of the model's JSON. An invented
		// or renumbered ID would either violate the segment_id foreign key or,
		// worse, silently overwrite another session's annotation, so only
		// accept IDs we actually sent.
		if _, valid := validIDs[seg.ID]; !valid {
			dropped++
			continue
		}
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
		switch a.Classification {
		case "narrative", "table_talk", "ambiguous":
		default:
			a.Classification = "narrative"
		}
		dbAnnotations = append(dbAnnotations, a)
		aCopy := a
		annotationMap[seg.ID] = &aCopy
	}

	if dropped > 0 {
		log.Printf("pipeline: dropped %d annotation(s) with unknown segment IDs", dropped)
	}
	if len(annotationMap) == 0 {
		log.Printf("pipeline: no annotations matched a real segment ID")
		return nil, nil
	}

	if err := b.store.InsertAnnotations(ctx, dbAnnotations); err != nil {
		log.Printf("pipeline: insert annotations: %v", err)
	}

	log.Printf("pipeline: annotated %d/%d segments across %d batches",
		len(annotationMap), len(inputs), (len(inputs)+batchSize-1)/batchSize)

	return annotationMap, segmentIDs
}

// buildAnnotatedTranscript produces a transcript string from merged segments
// and annotations. Table talk is marked with [TABLE TALK] so the summariser
// can deprioritize it while still having full context. Corrected text is used
// when available, scene boundaries are inserted, and NPC voices are labelled.
// segmentIDs holds the DB ID of merged[i] at index i, so annotations are
// looked up by segment identity. Matching positionally against a compacted
// list of annotations would shift every later annotation onto the wrong
// segment whenever one is missing (a failed batch, or a short LLM response).
// A nil/short segmentIDs slice simply means those segments are unannotated.
func buildAnnotatedTranscript(
	merged []transcribe.UserSegment,
	segmentIDs []int64,
	annotations map[int64]*storage.TranscriptAnnotation,
	dmName string,
) string {
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
		if i < len(segmentIDs) {
			ann = annotations[segmentIDs[i]]
		}

		// Mark table talk so the summariser can deprioritize it, but keep
		// it in the transcript so context isn't lost.
		isTableTalk := ann != nil && ann.Classification == "table_talk"

		// Insert scene boundary.
		if !isTableTalk && ann != nil && ann.Scene != nil && *ann.Scene != "" && *ann.Scene != lastScene {
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

		// Table talk: include but mark clearly so summariser deprioritizes it.
		if isTableTalk {
			flushMerge()
			ts := formatSeconds(seg.StartTime)
			fmt.Fprintf(&b, "[%s] [TABLE TALK] %s: %s\n", ts, name, text)
			continue
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

// interleaveTelegramIntoAnnotated inserts the session's stored Telegram
// messages into an already-built annotated transcript, in timestamp order.
// It reads from the DB so the live and reprocess paths produce the same
// transcript.
func (b *Bot) interleaveTelegramIntoAnnotated(
	ctx context.Context,
	session *storage.Session,
	campaign *storage.Campaign,
	transcript string,
	dmName string,
) string {
	tgMsgs, err := b.store.GetTelegramMessages(ctx, session.ID, false)
	if err != nil {
		log.Printf("pipeline: GetTelegramMessages: %v", err)
		return transcript
	}

	entries := telegramTranscriptEntries(session, campaign, tgMsgs, dmName)
	if len(entries) == 0 {
		return transcript
	}

	log.Printf("pipeline: interleaving %d Telegram message(s) into transcript", len(entries))
	return insertTelegramEntries(transcript, entries)
}

// telegramTranscriptEntries filters stored Telegram messages down to the
// session-relevant ones and converts them to transcript entries with elapsed
// timestamps.
func telegramTranscriptEntries(
	session *storage.Session,
	campaign *storage.Campaign,
	msgs []storage.TelegramMessage,
	dmName string,
) []transcribe.TelegramEntry {
	var telegramDMID int64
	if campaign != nil && campaign.TelegramDMUserID != nil {
		telegramDMID = *campaign.TelegramDMUserID
	}

	senderLabel := "DM"
	if dmName != "" {
		senderLabel = dmName
	}

	var entries []transcribe.TelegramEntry
	for _, m := range msgs {
		isDM := telegramDMID != 0 && m.FromUserID == telegramDMID
		if !telegram.IsRelevant(telegram.Message{FromID: m.FromUserID, Text: m.Text}, isDM) {
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
	return entries
}

// insertTelegramEntries splices Telegram entries into a rendered transcript,
// placing each one before the first "[HH:MM:SS]" line that starts later than
// it. Lines without a timestamp (scene headers, blank lines) are passed
// through untouched.
func insertTelegramEntries(transcript string, entries []transcribe.TelegramEntry) string {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].ElapsedSecs < entries[j].ElapsedSecs
	})

	trailingNewline := strings.HasSuffix(transcript, "\n")
	lines := strings.Split(strings.TrimSuffix(transcript, "\n"), "\n")

	out := make([]string, 0, len(lines)+len(entries))
	next := 0
	for _, line := range lines {
		if secs, ok := parseLineSeconds(line); ok {
			for next < len(entries) && entries[next].ElapsedSecs <= secs {
				out = append(out, formatTelegramLine(entries[next]))
				next++
			}
		}
		out = append(out, line)
	}
	for ; next < len(entries); next++ {
		out = append(out, formatTelegramLine(entries[next]))
	}

	joined := strings.Join(out, "\n")
	if trailingNewline {
		joined += "\n"
	}
	return joined
}

// formatTelegramLine renders a Telegram entry using the same "[Name via
// Telegram]" marker the summariser prompts tell the model to look for.
func formatTelegramLine(e transcribe.TelegramEntry) string {
	return fmt.Sprintf("[%s] [%s via Telegram]: %s", formatSeconds(e.ElapsedSecs), e.SenderName, e.Text)
}

// parseLineSeconds extracts the elapsed seconds from a transcript line that
// begins with an "[HH:MM:SS]" timestamp.
func parseLineSeconds(line string) (float64, bool) {
	if len(line) < 10 || line[0] != '[' || line[9] != ']' {
		return 0, false
	}
	var h, m, s int
	if _, err := fmt.Sscanf(line[1:9], "%02d:%02d:%02d", &h, &m, &s); err != nil {
		return 0, false
	}
	return float64(h*3600 + m*60 + s), true
}

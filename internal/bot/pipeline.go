package bot

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"discord-rpg-summariser/internal/audio"
	"discord-rpg-summariser/internal/diarize"
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/telegram"
	"discord-rpg-summariser/internal/transcribe"

	"github.com/bwmarrin/discordgo"
)

// runPipeline is executed asynchronously after recording stops. It transcribes
// each user's audio, merges segments chronologically (including any Telegram
// messages), summarises the transcript, persists everything to the database,
// and posts a notification.
func (b *Bot) runPipeline(sessionID int64, result stopResult) {
	userFiles := result.UserFiles
	telegramMsgs := result.TelegramMsgs
	ctx := summarise.WithSessionID(context.Background(), sessionID)

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
		log.Printf("pipeline: GetSession(%d): %v", sessionID, err)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		return
	}

	if len(userFiles) == 0 {
		log.Printf("pipeline: no user audio files for session %d", sessionID)
		b.store.UpdateSessionSummary(ctx, sessionID, "No audio was recorded.", nil)
		b.sendNotification(sessionID, "No audio was recorded.")
		return
	}

	// Load transcription model for the pipeline (may already be loaded for live).
	transcriber, err := b.acquireTranscriber()
	if err != nil {
		log.Printf("pipeline: failed to load transcriber: %v", err)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		return
	}
	defer b.releaseTranscriber()

	// Bias the transcriber toward campaign-specific vocabulary (character
	// names, entity names, quest names) so the ASR model is more likely to
	// recognise them correctly.
	if campaign, _ := b.store.GetCampaign(ctx, session.CampaignID); campaign != nil {
		transcriber.SetGameSystem(campaign.GameSystem)
	}
	transcriber.SetVocabulary(b.gatherCampaignVocabulary(ctx, session.CampaignID))

	// Transcribe each user's WAV, with diarization for shared mics.
	b.store.UpdateSessionStatus(ctx, sessionID, "transcribing")
	totalUsers := len(userFiles)
	b.progress.SetStage("transcribing", fmt.Sprintf("Transcribing audio (0 of %d users)", totalUsers))

	// Load shared mic config for this campaign.
	sharedMics, _ := b.store.GetSharedMics(ctx, session.CampaignID)
	sharedMicMap := make(map[string]storage.SharedMic, len(sharedMics))
	for _, m := range sharedMics {
		sharedMicMap[m.DiscordUserID] = m
	}

	// Pre-resolve character names and DM label for live transcript display.
	liveNames := make(map[string]string, len(userFiles))
	liveCampaign, _ := b.store.GetCampaign(ctx, session.CampaignID)
	for userID := range userFiles {
		if liveCampaign != nil && liveCampaign.DMUserID != nil && *liveCampaign.DMUserID == userID {
			liveNames[userID] = "DM"
			continue
		}
		if cn, _ := b.store.GetCharacterName(ctx, userID, session.CampaignID); cn != "" {
			liveNames[userID] = cn
			continue
		}
		liveNames[userID] = b.ResolveUsername(userID)
	}

	// Wire up intra-file progress if the transcriber supports it.
	// Capture the progress pointer at callback setup so a concurrent
	// pipeline that replaces b.progress can't cause us to nil-deref.
	type progressSetter interface {
		SetProgressCallback(func(float64))
	}
	progress := b.progress
	setIntraProgress := func(doneUsers int) {
		if ps, ok := transcriber.(progressSetter); ok {
			ps.SetProgressCallback(func(filePct float64) {
				if progress == nil {
					return
				}
				p := (float64(doneUsers) + filePct) / float64(totalUsers)
				progress.SetSubProgress(p)
			})
		}
	}

	userSegments := make(map[string][]transcribe.Segment, len(userFiles))
	doneUsers := 0
	pipelineTranscribeStart := time.Now()
	log.Printf("pipeline: starting transcription of %d users for session %d", totalUsers, session.ID)
	for userID, wavPath := range userFiles {
		userStart := time.Now()
		log.Printf("pipeline: transcribing user %s (%d of %d)", userID, doneUsers+1, totalUsers)
		setIntraProgress(doneUsers)

		// Check if we have pre-transcribed segments from incremental transcription.
		if preSegs, ok := result.PreTranscribed[userID]; ok && len(preSegs) > 0 {
			log.Printf("pipeline: using %d pre-transcribed segments for %s, processing remainder", len(preSegs), userID)
			userSegments[userID] = preSegs

			// Transcribe only the remaining unprocessed portion of the file.
			if processedBytes, ok := result.ProcessedOffsets[userID]; ok && processedBytes > 0 {
				remainSegs := b.transcribeRemainder(ctx, transcriber, wavPath, processedBytes)
				if len(remainSegs) > 0 {
					userSegments[userID] = append(userSegments[userID], remainSegs...)
					log.Printf("pipeline: added %d remainder segments for %s", len(remainSegs), userID)
				}
			}
		} else if mic, ok := sharedMicMap[userID]; ok {
			// Shared mic: diarize then attribute segments.
			b.transcribeSharedMic(ctx, transcriber, wavPath, mic, userSegments)
		} else {
			// Normal single-user transcription (no pre-transcribed data).
			segments, err := transcriber.TranscribeFile(ctx, wavPath)
			if err != nil {
				log.Printf("pipeline: transcribe user %s: %v", userID, err)
				doneUsers++
				b.progress.SetSubProgress(float64(doneUsers) / float64(totalUsers))
				continue
			}
			userSegments[userID] = segments
		}
		doneUsers++
		log.Printf("pipeline: user %s done in %s (%d of %d, %d segments)",
			userID, time.Since(userStart).Round(time.Second), doneUsers, totalUsers, len(userSegments[userID]))
		b.progress.SetDetail(fmt.Sprintf("Transcribing audio (%d of %d users)", doneUsers, totalUsers))
		b.progress.SetSubProgress(float64(doneUsers) / float64(totalUsers))

		// Stream completed segments to subscribers.
		for _, seg := range userSegments[userID] {
			name := liveNames[userID]
			if name == "" {
				name = b.ResolveUsername(userID)
			}
			b.progress.BroadcastTranscript(name, seg.Text, seg.StartTime, seg.EndTime)
		}

		// Reclaim ONNX inference memory between users.
		runtime.GC()
	}
	log.Printf("pipeline: all %d users transcribed in %s", totalUsers, time.Since(pipelineTranscribeStart).Round(time.Second))

	if len(userSegments) == 0 {
		log.Printf("pipeline: all transcriptions failed for session %d", sessionID)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		b.sendNotification(sessionID, "Transcription failed for all users.")
		return
	}

	// Auto-enroll voice embeddings for non-shared-mic users so future
	// shared-mic sessions can identify speakers by voice.
	if d := b.getDiarizer(); d != nil {
		for userID, wavPath := range userFiles {
			if _, ok := sharedMicMap[userID]; ok {
				continue
			}
			samples, err := audio.LoadAndResample(wavPath)
			if err != nil {
				continue
			}
			embedding, err := d.ExtractEmbedding(samples)
			if err != nil {
				log.Printf("pipeline: auto-enroll %s: %v", userID, err)
				continue
			}
			if err := b.store.UpsertSpeakerEnrollment(ctx, session.CampaignID, userID, embedding); err != nil {
				log.Printf("pipeline: save enrollment %s: %v", userID, err)
			}
		}
		log.Printf("pipeline: auto-enrolled voice embeddings for %d user(s)", len(userFiles)-len(sharedMicMap))
	}

	// Resolve campaign and DM for transcript labelling.
	campaign, _ := b.store.GetCampaign(ctx, session.CampaignID)
	var dmUserID string
	if campaign != nil && campaign.DMUserID != nil {
		dmUserID = *campaign.DMUserID
	}
	charNames := make(map[string]string, len(userSegments))
	for userID := range userSegments {
		if userID == dmUserID {
			charNames[userID] = "DM"
			continue
		}
		name, err := b.store.GetCharacterName(ctx, userID, session.CampaignID)
		if err != nil {
			log.Printf("pipeline: GetCharacterName(%s): %v", userID, err)
		}
		if name != "" {
			charNames[userID] = name
		}
	}

	// Load join offsets from the audio directory (written by the recorder as
	// users join). This is more robust than passing them through memory since
	// it also works if the bot crashed and the pipeline is re-run.
	joinOffsetSecs := audio.LoadJoinOffsets(session.AudioDir)

	// Generate the mixed-down audio file now that recording is finished.
	b.progress.SetStage("mixing", "Mixing audio tracks")
	mixedPath := filepath.Join(session.AudioDir, "mixed.wav")
	if err := audio.MixAndNormalize(userFiles, mixedPath, joinOffsetSecs); err != nil {
		log.Printf("pipeline: mix audio: %v", err)
	}

	// Merge voice segments with join offsets so late joiners are correctly placed.
	merged := transcribe.MergeTranscripts(userSegments, charNames, joinOffsetSecs)

	// Persist transcript segments (store only user_id; character names are
	// resolved from mappings at display time so they stay up to date).
	var dbSegments []storage.TranscriptSegment
	for _, seg := range merged {
		dbSegments = append(dbSegments, storage.TranscriptSegment{
			SessionID: sessionID,
			UserID:    seg.UserID,
			StartTime: seg.StartTime,
			EndTime:   seg.EndTime,
			Text:      seg.Text,
		})
	}
	if err := b.store.InsertSegments(ctx, dbSegments); err != nil {
		log.Printf("pipeline: InsertSegments: %v", err)
	}

	// Resolve DM display name for Telegram filtering and LLM prompts.
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
		log.Printf("pipeline: annotation failed for session %d, aborting pipeline", sessionID)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		b.sendNotification(sessionID, "Transcript annotation failed. Please retry.")
		return
	}

	transcript := buildAnnotatedTranscript(merged, annotations, dmName)
	log.Printf("pipeline: annotated transcript built (%d annotations, %d narrative, %d table_talk)",
		len(annotations),
		countClassification(annotations, "narrative"),
		countClassification(annotations, "table_talk"))

	// Interleave Telegram messages.
	transcript = b.interleaveTelegramIntoAnnotated(ctx, session, campaign, transcript, telegramMsgs, dmName)

	// Summarise.
	b.store.UpdateSessionStatus(ctx, sessionID, "summarising")
	b.progress.SetStage("summarising", "Generating summary")

	sumResult, err := b.summariser.Summarise(ctx, transcript, "", dmName)
	if err != nil {
		log.Printf("pipeline: summarise: %v", err)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		b.sendNotification(sessionID, "Summarisation failed.")
		return
	}

	// Persist summary.
	if err := b.store.UpdateSessionSummary(ctx, sessionID, sumResult.Summary, sumResult.KeyEvents); err != nil {
		log.Printf("pipeline: UpdateSessionSummary: %v", err)
		b.store.UpdateSessionStatus(ctx, sessionID, "failed")
		return
	}

	b.sendNotification(sessionID, sumResult.Summary)

	// Run extraction stages in parallel — they are all independent and non-fatal.
	b.progress.SetStage("extracting", "Extracting title, entities, quests, and combat")

	var extractWg sync.WaitGroup
	extractWg.Add(4)

	go func() {
		defer extractWg.Done()
		b.extractTitleAndQuotes(ctx, session, sessionID, transcript, sumResult.Summary, dmName)
	}()

	go func() {
		defer extractWg.Done()
		b.extractEntities(ctx, session, sessionID, transcript, sumResult.Summary, dmName)
	}()

	go func() {
		defer extractWg.Done()
		b.extractQuests(ctx, session, sessionID, transcript, sumResult.Summary, dmName)
	}()

	go func() {
		defer extractWg.Done()
		// Combat then creatures (creatures depend on combat encounters in DB).
		b.extractCombat(ctx, session, sessionID, transcript, sumResult.Summary, dmName)
		b.extractCreatures(ctx, session, sessionID, transcript, sumResult.Summary, dmName)
	}()

	extractWg.Wait()

	// Generate vector embeddings after extractions complete (embeds entities and quests).
	b.progress.SetStage("generating embeddings", "Generating embeddings")
	b.generateEmbeddings(ctx, session, sessionID, merged, sumResult.Summary, dmName)

	b.progress.Complete()
}

// transcribeRemainder transcribes only the unprocessed portion of a WAV file,
// starting from the given byte offset.
func (b *Bot) transcribeRemainder(ctx context.Context, transcriber transcribe.Transcriber, wavPath string, processedBytes int64) []transcribe.Segment {
	startTimeSec := float64(processedBytes) / (48000 * 2)
	var segments []transcribe.Segment

	err := audio.StreamResample(wavPath, func(samples []float32, offsetSeconds float64) error {
		// Skip chunks before our processed offset.
		if offsetSeconds < startTimeSec-1.0 {
			return nil
		}

		segs, err := transcriber.TranscribeChunk(ctx, samples,
			time.Duration(offsetSeconds*float64(time.Second)), "")
		if err != nil {
			return err
		}

		for _, seg := range segs {
			if seg.StartTime >= startTimeSec {
				segments = append(segments, seg)
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("pipeline: transcribe remainder error: %v", err)
	}
	return segments
}

// gatherCampaignVocabulary collects campaign-specific proper nouns for
// transcription biasing: character names, entity names, and quest names.
func (b *Bot) gatherCampaignVocabulary(ctx context.Context, campaignID int64) []string {
	seen := make(map[string]struct{})
	var words []string
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		words = append(words, name)
	}

	charMappings, _ := b.store.GetCharacterMappings(ctx, campaignID)
	for _, m := range charMappings {
		add(m.CharacterName)
	}

	entities, _ := b.store.ListEntities(ctx, campaignID, "", "", 1000, 0)
	for _, e := range entities {
		add(e.Name)
	}

	quests, _ := b.store.ListQuests(ctx, campaignID, "")
	for _, q := range quests {
		add(q.Name)
	}

	return words
}

// buildTranscriptWithTelegram persists Telegram messages to the DB, filters
// them, and returns a formatted transcript with voice segments and Telegram
// messages interleaved chronologically.
func (b *Bot) buildTranscriptWithTelegram(
	ctx context.Context,
	session *storage.Session,
	campaign *storage.Campaign,
	merged []transcribe.UserSegment,
	telegramMsgs []telegram.Message,
	dmName string,
) string {
	// If no Telegram messages, just format voice segments.
	if len(telegramMsgs) == 0 {
		return transcribe.FormatTranscript(merged)
	}

	// Determine the Telegram DM user ID for filtering.
	var telegramDMID int64
	if campaign != nil && campaign.TelegramDMUserID != nil {
		telegramDMID = *campaign.TelegramDMUserID
	}

	// Persist all Telegram messages to DB.
	var dbMsgs []storage.TelegramMessage
	for _, m := range telegramMsgs {
		isDM := telegramDMID != 0 && m.FromID == telegramDMID
		dbMsgs = append(dbMsgs, storage.TelegramMessage{
			SessionID:     session.ID,
			TelegramMsgID: m.MessageID,
			FromUserID:    m.FromID,
			FromUsername:  m.FromUsername,
			FromDisplay:   m.FromDisplay,
			Text:          m.Text,
			SentAt:        m.Timestamp,
			IsDM:          isDM,
		})
	}
	if err := b.store.InsertTelegramMessages(ctx, dbMsgs); err != nil {
		log.Printf("pipeline: InsertTelegramMessages: %v", err)
	}

	// Filter: only DM messages that pass relevance check.
	var entries []transcribe.TelegramEntry
	senderLabel := "DM"
	if dmName != "" {
		senderLabel = dmName
	}

	for _, m := range telegramMsgs {
		isDM := telegramDMID != 0 && m.FromID == telegramDMID
		if !telegram.IsRelevant(m, isDM) {
			continue
		}
		elapsed := m.Timestamp.Sub(session.StartedAt).Seconds()
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

	log.Printf("pipeline: interleaving %d Telegram messages into transcript", len(entries))
	return transcribe.FormatTranscriptWithTelegram(merged, entries)
}

// transcribeSharedMic transcribes a shared-mic WAV file and attributes each
// segment to the correct speaker using per-segment voice embeddings — the same
// approach that works well in live transcription.
func (b *Bot) transcribeSharedMic(ctx context.Context, transcriber transcribe.Transcriber, wavPath string, mic storage.SharedMic, userSegments map[string][]transcribe.Segment) {
	d := b.getDiarizer()
	if d == nil {
		log.Printf("pipeline: diarizer not available, treating shared mic user %s as single speaker", mic.DiscordUserID)
		segments, err := transcriber.TranscribeFile(ctx, wavPath)
		if err != nil {
			log.Printf("pipeline: transcribe shared mic user %s: %v", mic.DiscordUserID, err)
			return
		}
		userSegments[mic.DiscordUserID] = segments
		return
	}

	// Load enrolled voice embeddings for speaker identification.
	enrollments := make(map[string][]float32)
	if enroll, _ := b.store.GetSpeakerEnrollment(ctx, mic.CampaignID, mic.DiscordUserID); enroll != nil {
		enrollments[mic.DiscordUserID] = enroll.Embedding
	}
	if enroll, _ := b.store.GetSpeakerEnrollment(ctx, mic.CampaignID, mic.PartnerUserID); enroll != nil {
		enrollments[mic.PartnerUserID] = enroll.Embedding
	}

	if len(enrollments) == 0 {
		log.Printf("pipeline: no voice enrollments for shared mic %s, falling back to single speaker", mic.DiscordUserID)
		segments, err := transcriber.TranscribeFile(ctx, wavPath)
		if err != nil {
			log.Printf("pipeline: transcribe shared mic user %s: %v", mic.DiscordUserID, err)
			return
		}
		userSegments[mic.DiscordUserID] = segments
		return
	}

	// Load the full audio at 16kHz for per-segment embedding extraction.
	samples16k, err := audio.LoadAndResample(wavPath)
	if err != nil {
		log.Printf("pipeline: resample for speaker ID %s: %v", mic.DiscordUserID, err)
		return
	}

	// Transcribe the full audio first.
	allSegments, err := transcriber.TranscribeFile(ctx, wavPath)
	if err != nil {
		log.Printf("pipeline: transcribe shared mic %s: %v", mic.DiscordUserID, err)
		return
	}

	// Attribute each segment by extracting a voice embedding from that
	// segment's audio and comparing against enrolled embeddings —
	// the same approach used in live transcription.
	ownerCount, partnerCount := 0, 0
	for _, seg := range allSegments {
		// Extract the audio for this segment at 16kHz.
		startIdx := int(seg.StartTime * 16000)
		endIdx := int(seg.EndTime * 16000)
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx > len(samples16k) {
			endIdx = len(samples16k)
		}
		if endIdx <= startIdx {
			userSegments[mic.DiscordUserID] = append(userSegments[mic.DiscordUserID], seg)
			continue
		}

		segAudio := samples16k[startIdx:endIdx]

		// Extract embedding for this segment.
		emb, err := d.ExtractEmbedding(segAudio)
		if err != nil {
			// Can't identify — default to mic owner.
			userSegments[mic.DiscordUserID] = append(userSegments[mic.DiscordUserID], seg)
			continue
		}

		// Find the best matching enrolled speaker.
		var bestID string
		bestSim := -1.0
		for uid, enrolled := range enrollments {
			sim := diarize.CosineSimilarity(emb, enrolled)
			if sim > bestSim {
				bestSim = sim
				bestID = uid
			}
		}

		if bestSim < 0.3 || bestID == mic.DiscordUserID {
			userSegments[mic.DiscordUserID] = append(userSegments[mic.DiscordUserID], seg)
			ownerCount++
		} else {
			userSegments[mic.PartnerUserID] = append(userSegments[mic.PartnerUserID], seg)
			partnerCount++
		}
	}

	log.Printf("pipeline: shared mic %s: %d segments attributed (%d owner, %d partner) using per-segment embeddings",
		mic.DiscordUserID, len(allSegments), ownerCount, partnerCount)
}

// ---------------------------------------------------------------------------
// Entity extraction
// ---------------------------------------------------------------------------

func (b *Bot) extractEntities(ctx context.Context, session *storage.Session, sessionID int64, transcript, summary, dmName string) {
	extractor, ok := b.summariser.(summarise.EntityExtractor)
	if !ok {
		return
	}

	existing, _ := b.store.ListEntities(ctx, session.CampaignID, "", "", 1000, 0)
	var existingNames []string
	for _, e := range existing {
		existingNames = append(existingNames, fmt.Sprintf("%s (%s)", e.Name, e.Type))
	}

	// Collect player character names so the LLM doesn't extract them as NPCs.
	charMappings, _ := b.store.GetCharacterMappings(ctx, session.CampaignID)
	var playerCharacters []string
	for _, m := range charMappings {
		playerCharacters = append(playerCharacters, m.CharacterName)
	}

	// Ensure PC entities exist for all player characters before extraction.
	pcEntityIDs, err := b.store.EnsurePCEntities(ctx, session.CampaignID, playerCharacters)
	if err != nil {
		log.Printf("pipeline: ensure PC entities: %v", err)
	}

	extraction, err := extractor.ExtractEntities(ctx, transcript, summary, existingNames, dmName, playerCharacters)
	if err != nil {
		log.Printf("pipeline: entity extraction: %v", err)
		return
	}

	// Persist entities and notes
	entityIDs := make(map[string]int64) // "name|type" -> ID
	for _, e := range extraction.Entities {
		id, err := b.store.UpsertEntity(ctx, session.CampaignID, e.Name, e.Type, e.Description)
		if err != nil {
			log.Printf("pipeline: upsert entity %q: %v", e.Name, err)
			continue
		}
		entityIDs[e.Name+"|"+e.Type] = id
		if e.Status != "" {
			if err := b.store.UpdateEntityStatus(ctx, id, e.Status, e.CauseOfDeath); err != nil {
				log.Printf("pipeline: update entity status %q: %v", e.Name, err)
			}
		}
		if e.Notes != "" {
			if err := b.store.AddEntityNote(ctx, id, sessionID, e.Notes); err != nil {
				log.Printf("pipeline: add note for %q: %v", e.Name, err)
			}
		}
	}

	// Resolve parent_place for place entities and set parent hierarchy.
	for _, e := range extraction.Entities {
		if e.Type != "place" || e.ParentPlace == "" {
			continue
		}
		childID := entityIDs[e.Name+"|"+e.Type]
		if childID == 0 {
			continue
		}
		parentID := findEntityID(entityIDs, e.ParentPlace)
		if parentID == 0 {
			// Try to find the parent by name as a place entity in the DB.
			parentEntity, _ := b.store.GetEntityByName(ctx, session.CampaignID, e.ParentPlace, "place")
			if parentEntity != nil {
				parentID = parentEntity.ID
			}
		}
		if parentID != 0 {
			if err := b.store.SetEntityParent(ctx, childID, parentID); err != nil {
				log.Printf("pipeline: set parent for %q: %v", e.Name, err)
			}
		}
	}

	// Add PC entity IDs so relationships referencing PCs can be resolved.
	for name, id := range pcEntityIDs {
		entityIDs[name+"|pc"] = id
	}

	// Persist relationships
	for _, r := range extraction.Relationships {
		sourceID := findEntityID(entityIDs, r.Source)
		targetID := findEntityID(entityIDs, r.Target)
		if sourceID == 0 || targetID == 0 {
			continue
		}
		sid := sessionID
		if err := b.store.UpsertEntityRelationship(ctx, session.CampaignID, sourceID, targetID, r.Relationship, r.Description, &sid); err != nil {
			log.Printf("pipeline: upsert relationship %q->%q: %v", r.Source, r.Target, err)
		}
	}

	log.Printf("pipeline: extracted %d entities, %d relationships", len(extraction.Entities), len(extraction.Relationships))

	// Link entity references to transcript segments.
	b.linkEntityReferences(ctx, sessionID, entityIDs)
}

// linkEntityReferences scans transcript segments for mentions of entities and
// inserts entity_references rows linking them.
func (b *Bot) linkEntityReferences(ctx context.Context, sessionID int64, entityIDs map[string]int64) {
	segments, err := b.store.GetTranscript(ctx, sessionID)
	if err != nil {
		log.Printf("pipeline: linkEntityReferences: get transcript: %v", err)
		return
	}
	if len(segments) == 0 {
		return
	}

	// Build a map of entity name -> entity ID, skipping names shorter than 3 chars.
	nameToID := make(map[string]int64)
	for key, id := range entityIDs {
		parts := strings.SplitN(key, "|", 2)
		name := parts[0]
		if len([]rune(name)) < 3 {
			continue
		}
		nameToID[name] = id
	}

	if len(nameToID) == 0 {
		return
	}

	var refs []storage.EntityReference
	for i := range segments {
		seg := &segments[i]
		matches := findEntityMentions(seg.Text, nameToID)
		for entityName, entityID := range matches {
			ctx := truncateContext(seg.Text, entityName, 200)
			segID := seg.ID
			refs = append(refs, storage.EntityReference{
				EntityID:  entityID,
				SessionID: sessionID,
				SegmentID: &segID,
				Context:   ctx,
			})
		}
	}

	if len(refs) == 0 {
		return
	}

	if err := b.store.InsertEntityReferences(ctx, refs); err != nil {
		log.Printf("pipeline: linkEntityReferences: insert: %v", err)
		return
	}

	log.Printf("pipeline: linked %d entity references for session %d", len(refs), sessionID)
}

// findEntityMentions performs case-insensitive word-boundary matching of entity
// names against the given text. Returns a map of matched entity name -> ID.
func findEntityMentions(text string, nameToID map[string]int64) map[string]int64 {
	matches := make(map[string]int64)
	lower := strings.ToLower(text)

	for name, id := range nameToID {
		pattern := `(?i)\b` + regexp.QuoteMeta(name) + `\b`
		re, err := regexp.Compile(pattern)
		if err != nil {
			// If the name has characters that break regex even after quoting,
			// fall back to simple case-insensitive contains with boundary check.
			if containsWordBoundary(lower, strings.ToLower(name)) {
				matches[name] = id
			}
			continue
		}
		if re.MatchString(text) {
			matches[name] = id
		}
	}

	return matches
}

// containsWordBoundary checks if text contains substr at a word boundary.
func containsWordBoundary(text, substr string) bool {
	idx := strings.Index(text, substr)
	if idx < 0 {
		return false
	}
	// Check left boundary.
	if idx > 0 {
		r := rune(text[idx-1])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	// Check right boundary.
	end := idx + len(substr)
	if end < len(text) {
		r := rune(text[end])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// truncateContext returns a context snippet from text around the entity name,
// truncated to maxLen characters.
func truncateContext(text, entityName string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	// Try to center around the entity mention.
	lower := strings.ToLower(text)
	idx := strings.Index(lower, strings.ToLower(entityName))
	if idx < 0 {
		return text[:maxLen]
	}
	start := idx - (maxLen-len(entityName))/2
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(text) {
		end = len(text)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}
	return text[start:end]
}

func findEntityID(ids map[string]int64, name string) int64 {
	// Try to find by name with any type
	for key, id := range ids {
		if strings.HasPrefix(key, name+"|") {
			return id
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// Quest extraction
// ---------------------------------------------------------------------------

func (b *Bot) extractQuests(ctx context.Context, session *storage.Session, sessionID int64, transcript, summary, dmName string) {
	extractor, ok := b.summariser.(summarise.QuestExtractor)
	if !ok {
		return
	}

	existing, _ := b.store.ListQuests(ctx, session.CampaignID, "")
	var existingNames []string
	for _, q := range existing {
		existingNames = append(existingNames, q.Name)
	}

	extraction, err := extractor.ExtractQuests(ctx, transcript, summary, existingNames, dmName)
	if err != nil {
		log.Printf("pipeline: quest extraction: %v", err)
		return
	}

	for _, q := range extraction.Quests {
		questID, err := b.store.UpsertQuest(ctx, session.CampaignID, q.Name, q.Description, q.Status, q.Giver)
		if err != nil {
			log.Printf("pipeline: upsert quest %q: %v", q.Name, err)
			continue
		}
		var newStatus *string
		if q.Status == "completed" || q.Status == "failed" {
			newStatus = &q.Status
			if err := b.store.UpdateQuestStatus(ctx, questID, q.Status); err != nil {
				log.Printf("pipeline: update quest status %q: %v", q.Name, err)
			}
		}
		if q.Update != "" {
			if err := b.store.AddQuestUpdate(ctx, questID, sessionID, q.Update, newStatus); err != nil {
				log.Printf("pipeline: add quest update for %q: %v", q.Name, err)
			}
		}
	}

	log.Printf("pipeline: extracted %d quests", len(extraction.Quests))
}

// ---------------------------------------------------------------------------
// Combat extraction
// ---------------------------------------------------------------------------

func (b *Bot) extractCombat(ctx context.Context, session *storage.Session, sessionID int64, transcript, summary, dmName string) {
	extractor, ok := b.summariser.(summarise.CombatExtractor)
	if !ok {
		return
	}

	// Collect player character names.
	charMappings, _ := b.store.GetCharacterMappings(ctx, session.CampaignID)
	var playerCharacters []string
	for _, m := range charMappings {
		playerCharacters = append(playerCharacters, m.CharacterName)
	}

	extraction, err := extractor.ExtractCombat(ctx, transcript, summary, dmName, playerCharacters)
	if err != nil {
		log.Printf("pipeline: combat extraction: %v", err)
		return
	}

	for _, enc := range extraction.Encounters {
		encID, err := b.store.InsertCombatEncounter(ctx, storage.CombatEncounter{
			SessionID:  sessionID,
			CampaignID: session.CampaignID,
			Name:       enc.Name,
			StartTime:  enc.StartTime,
			EndTime:    enc.EndTime,
			Summary:    enc.Summary,
		})
		if err != nil {
			log.Printf("pipeline: insert combat encounter %q: %v", enc.Name, err)
			continue
		}

		var actions []storage.CombatAction
		for _, a := range enc.Actions {
			actions = append(actions, storage.CombatAction{
				Actor:      a.Actor,
				ActionType: a.ActionType,
				Target:     a.Target,
				Detail:     a.Detail,
				Damage:     a.Damage,
				Round:      a.Round,
				Timestamp:  a.Timestamp,
			})
		}
		if err := b.store.InsertCombatActions(ctx, encID, actions); err != nil {
			log.Printf("pipeline: insert combat actions for %q: %v", enc.Name, err)
		}
	}

	log.Printf("pipeline: extracted %d combat encounters", len(extraction.Encounters))
}

// ---------------------------------------------------------------------------
// Creature extraction (bestiary)
// ---------------------------------------------------------------------------

func (b *Bot) extractCreatures(ctx context.Context, session *storage.Session, sessionID int64, transcript, summary, dmName string) {
	extractor, ok := b.summariser.(summarise.CreatureExtractor)
	if !ok {
		return
	}

	// Get combat encounters for this session — if none, skip.
	encounters, err := b.store.GetCombatEncounters(ctx, sessionID)
	if err != nil || len(encounters) == 0 {
		return
	}

	// Collect player character names.
	charMappings, _ := b.store.GetCharacterMappings(ctx, session.CampaignID)
	var playerCharacters []string
	for _, m := range charMappings {
		playerCharacters = append(playerCharacters, m.CharacterName)
	}

	// Build combat encounter data for the LLM prompt.
	var combatData []summarise.CombatExtractedEncounter
	for _, enc := range encounters {
		actions, _ := b.store.GetCombatActions(ctx, enc.ID)
		var extractedActions []summarise.CombatExtractedAction
		for _, a := range actions {
			extractedActions = append(extractedActions, summarise.CombatExtractedAction{
				Actor:      a.Actor,
				ActionType: a.ActionType,
				Target:     a.Target,
				Detail:     a.Detail,
				Damage:     a.Damage,
				Round:      a.Round,
				Timestamp:  a.Timestamp,
			})
		}
		combatData = append(combatData, summarise.CombatExtractedEncounter{
			Name:      enc.Name,
			StartTime: enc.StartTime,
			EndTime:   enc.EndTime,
			Summary:   enc.Summary,
			Actions:   extractedActions,
		})
	}

	extraction, err := extractor.ExtractCreatures(ctx, transcript, summary, combatData, playerCharacters)
	if err != nil {
		log.Printf("pipeline: creature extraction: %v", err)
		return
	}

	nameToEntityID := make(map[string]int64)

	for _, c := range extraction.Creatures {
		status := c.Status
		if status == "" {
			status = "unknown"
		}

		entityID, err := b.store.UpsertEntity(ctx, session.CampaignID, c.Name, "creature", c.Description)
		if err != nil {
			log.Printf("pipeline: upsert creature entity %q: %v", c.Name, err)
			continue
		}

		if status != "unknown" {
			b.store.UpdateEntityStatus(ctx, entityID, status, "")
		}

		err = b.store.UpsertCreatureStats(ctx, entityID, storage.CreatureStats{
			CreatureType:    c.CreatureType,
			ChallengeRating: c.ChallengeRating,
			ArmorClass:      c.ArmorClass,
			HitPoints:       c.HitPoints,
			Abilities:       c.Abilities,
			Loot:            c.Loot,
		})
		if err != nil {
			log.Printf("pipeline: upsert creature stats %q: %v", c.Name, err)
		}

		b.store.AddEntityNote(ctx, entityID, sessionID, fmt.Sprintf("Encountered in combat — %s", status))
		nameToEntityID[c.Name] = entityID
	}

	// Also map PC names to entity IDs for combat action linking.
	for _, m := range charMappings {
		pcEntity, err := b.store.GetEntityByName(ctx, session.CampaignID, m.CharacterName, "pc")
		if err == nil && pcEntity != nil {
			nameToEntityID[m.CharacterName] = pcEntity.ID
		}
	}

	// Link combat actions to entity IDs.
	if len(nameToEntityID) > 0 {
		if err := b.store.LinkCombatActorsToEntities(ctx, session.CampaignID, nameToEntityID); err != nil {
			log.Printf("pipeline: link combat actors: %v", err)
		}
	}

	log.Printf("pipeline: extracted %d creatures for bestiary", len(extraction.Creatures))
}

// ---------------------------------------------------------------------------
// Title and quote extraction
// ---------------------------------------------------------------------------

func (b *Bot) extractTitleAndQuotes(ctx context.Context, session *storage.Session, sessionID int64, transcript, summary, dmName string) {
	extractor, ok := b.summariser.(summarise.TitleAndQuotesExtractor)
	if !ok {
		return
	}

	result, err := extractor.ExtractTitleAndQuotes(ctx, transcript, summary, dmName)
	if err != nil {
		log.Printf("pipeline: title/quotes extraction: %v", err)
		return
	}

	if result.Title != "" {
		if err := b.store.UpdateSessionTitle(ctx, sessionID, result.Title); err != nil {
			log.Printf("pipeline: UpdateSessionTitle: %v", err)
		}
	}

	if len(result.Quotes) > 0 {
		var quotes []storage.SessionQuote
		for _, q := range result.Quotes {
			var tone *string
			if q.Tone != "" {
				tone = &q.Tone
			}
			quotes = append(quotes, storage.SessionQuote{
				SessionID: sessionID,
				Speaker:   q.Speaker,
				Text:      q.Text,
				StartTime: q.StartTime,
				Tone:      tone,
			})
		}
		if err := b.store.InsertQuotes(ctx, quotes); err != nil {
			log.Printf("pipeline: InsertQuotes: %v", err)
		}
	}

	log.Printf("pipeline: extracted title %q and %d quotes", result.Title, len(result.Quotes))
}

// sendNotification posts an embed to the configured notification channel with a
// summary preview and a link to the full web view.
func (b *Bot) sendNotification(sessionID int64, summary string) {
	channelID := b.config.Discord.NotificationChannel
	if channelID == "" {
		return
	}

	preview := summary
	if len(preview) > 1024 {
		preview = preview[:1021] + "..."
	}

	webURL := fmt.Sprintf("%s/sessions/%d", b.webBaseURL, sessionID)

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Session #%d Summary", sessionID),
		Description: preview,
		URL:         webURL,
		Color:       0x7C3AED,
		Footer:      &discordgo.MessageEmbedFooter{Text: "View full transcript and summary on the web."},
	}

	_, err := b.session.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		log.Printf("sendNotification: %v", err)
	}
}

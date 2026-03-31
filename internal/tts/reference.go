package tts

import (
	"context"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"strings"

	"discord-rpg-summariser/internal/audio"
	"discord-rpg-summariser/internal/storage"
)

// ReferenceClip holds a short audio clip and its transcript for voice cloning.
type ReferenceClip struct {
	Samples    []float32
	SampleRate int
	Text       string
}

const (
	minRefDur     = 5.0  // minimum reference clip duration in seconds
	maxRefDur     = 10.0 // maximum reference clip duration in seconds
	idealDur      = 8.0  // ideal reference clip duration (longer = better voice embedding)
	sampleRate48k = 48000
	maxSessions   = 5 // search this many recent sessions for the best clip
)

// ExtractReference finds a suitable reference audio clip for the given user
// by searching across their recent session recordings. For shared mic users,
// it extracts audio from the correct time ranges in the mic owner's WAV file.
//
// It scores candidates by duration, audio quality (RMS energy, consistency),
// and transcript quality to find a clip where the user is speaking clearly.
func ExtractReference(store *storage.Store, campaignID int64, userID string) (*ReferenceClip, error) {
	ctx := context.Background()

	sessions, err := store.GetUserSessionsWithAudio(ctx, campaignID, userID, maxSessions)
	if err != nil {
		return nil, fmt.Errorf("find sessions with audio: %w", err)
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no session with audio found for user %s", userID)
	}

	// Determine the WAV user ID (may differ for shared mic partners).
	wavUserID := userID
	mics, err := store.GetSharedMics(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("get shared mics: %w", err)
	}
	for _, m := range mics {
		if m.PartnerUserID == userID {
			wavUserID = m.DiscordUserID
			break
		}
	}

	// Search across all recent sessions for the best candidate.
	var allCandidates []candidate

	for _, sess := range sessions {
		segments, err := store.GetTranscript(ctx, sess.ID)
		if err != nil {
			log.Printf("tts reference: get transcript for session %d: %v", sess.ID, err)
			continue
		}

		var userSegs []storage.TranscriptSegment
		for _, seg := range segments {
			if seg.UserID == userID {
				userSegs = append(userSegs, seg)
			}
		}
		if len(userSegs) == 0 {
			continue
		}

		wavPath := filepath.Join(sess.AudioDir, wavUserID+".wav")
		allSamples, err := audio.LoadRaw48k(wavPath)
		if err != nil {
			log.Printf("tts reference: load wav %s: %v", wavPath, err)
			continue
		}

		// Get join offset for correct WAV positioning.
		joinOffset := 0.0
		offsets := audio.LoadJoinOffsets(sess.AudioDir)
		if offsets != nil {
			joinOffset = offsets[wavUserID]
		}

		candidates := buildCandidates(userSegs, allSamples, joinOffset, sess.ID)
		allCandidates = append(allCandidates, candidates...)
	}

	if len(allCandidates) == 0 {
		return nil, fmt.Errorf("no viable reference clips for user %s across %d sessions", userID, len(sessions))
	}

	// Pick the best candidate.
	best := allCandidates[0]
	for _, c := range allCandidates[1:] {
		if c.score > best.score {
			best = c
		}
	}

	dur := best.endTime - best.startTime
	wps := float64(len(strings.Fields(best.text))) / dur
	log.Printf("tts reference: selected clip for user %s from session %d: "+
		"%.1f-%.1fs (%.1fs) score=%.3f rms=%.4f wps=%.1f text=%q",
		userID, best.sessionID, best.startTime, best.endTime,
		dur, best.score, best.rms, wps, best.text)

	// Load the winning session's audio and extract.
	sess := findSession(sessions, best.sessionID)
	wavPath := filepath.Join(sess.AudioDir, wavUserID+".wav")
	allSamples, err := audio.LoadRaw48k(wavPath)
	if err != nil {
		return nil, fmt.Errorf("load wav for extraction: %w", err)
	}

	joinOffset := 0.0
	offsets := audio.LoadJoinOffsets(sess.AudioDir)
	if offsets != nil {
		joinOffset = offsets[wavUserID]
	}

	wavStart := best.startTime - joinOffset
	wavEnd := best.endTime - joinOffset
	extracted := audio.ExtractTimeRange(allSamples, sampleRate48k, wavStart, wavEnd)

	return &ReferenceClip{
		Samples:    extracted,
		SampleRate: sampleRate48k,
		Text:       best.text,
	}, nil
}

func findSession(sessions []storage.Session, id int64) *storage.Session {
	for i := range sessions {
		if sessions[i].ID == id {
			return &sessions[i]
		}
	}
	return &sessions[0]
}

type candidate struct {
	startTime float64
	endTime   float64
	text      string
	score     float64
	rms       float64
	sessionID int64
}

// buildCandidates generates scored candidate clips from transcript segments.
// joinOffset is subtracted from segment timestamps to get WAV-relative positions.
// It considers whole segments, concatenated segments, and also trims within
// longer segments using a sliding window to find the cleanest sub-region.
func buildCandidates(segs []storage.TranscriptSegment, allSamples []float32, joinOffset float64, sessionID int64) []candidate {
	var candidates []candidate

	extractForScoring := func(start, end float64) []float32 {
		return audio.ExtractTimeRange(allSamples, sampleRate48k, start-joinOffset, end-joinOffset)
	}

	addCandidate := func(start, end float64, text string) {
		dur := end - start
		if dur < 3.0 || dur > maxRefDur {
			return
		}
		text = cleanReferenceText(text)
		if text == "" {
			return
		}
		samples := extractForScoring(start, end)
		s, r := scoreClip(samples, dur, text)
		candidates = append(candidates, candidate{
			startTime: start,
			endTime:   end,
			text:      text,
			score:     s,
			rms:       r,
			sessionID: sessionID,
		})
	}

	// Score individual segments.
	for _, seg := range segs {
		dur := seg.EndTime - seg.StartTime
		if dur < 3.0 {
			continue
		}
		addCandidate(seg.StartTime, seg.EndTime, seg.Text)
	}

	// Concatenate consecutive segments to build longer clips.
	for i := 0; i < len(segs); i++ {
		text := segs[i].Text
		startTime := segs[i].StartTime
		endTime := segs[i].EndTime

		for j := i + 1; j < len(segs); j++ {
			gap := segs[j].StartTime - endTime
			if gap > 2.0 {
				break
			}
			endTime = segs[j].EndTime
			text += " " + segs[j].Text
			totalDur := endTime - startTime
			if totalDur > maxRefDur {
				break
			}
			if totalDur >= 3.0 {
				addCandidate(startTime, endTime, text)
				break
			}
		}
	}

	// Sliding window trimming: for segments longer than 5s, try sub-windows
	// to find a cleaner region inside. This catches cases where a segment
	// starts or ends with mumbling/silence but has clear speech in the middle.
	for _, seg := range segs {
		dur := seg.EndTime - seg.StartTime
		if dur < 5.0 {
			continue // too short to trim meaningfully
		}

		// Try windows of 5s, 6s, 7s stepping by 1s.
		for winDur := 5.0; winDur <= math.Min(dur, maxRefDur); winDur += 1.0 {
			for offset := 0.0; offset+winDur <= dur+0.01; offset += 1.0 {
				winStart := seg.StartTime + offset
				winEnd := winStart + winDur

				// Estimate word count for this window proportionally.
				words := strings.Fields(seg.Text)
				totalWords := len(words)
				if totalWords == 0 {
					continue
				}
				fracStart := offset / dur
				fracEnd := (offset + winDur) / dur
				wordStart := int(fracStart * float64(totalWords))
				wordEnd := int(fracEnd * float64(totalWords))
				if wordEnd > totalWords {
					wordEnd = totalWords
				}
				if wordEnd <= wordStart {
					continue
				}
				winText := strings.Join(words[wordStart:wordEnd], " ")

				addCandidate(winStart, winEnd, winText)
			}
		}
	}

	return candidates
}

// scoreClip scores a candidate reference clip. Higher is better.
// Returns the composite score and the RMS energy for logging.
func scoreClip(samples []float32, dur float64, text string) (float64, float64) {
	if len(samples) == 0 {
		return 0, 0
	}

	rms := rmsEnergy(samples)

	// Energy score: reject silence/mumbling and clipping.
	var energyScore float64
	switch {
	case rms < 0.005:
		energyScore = 0
	case rms < 0.015:
		energyScore = 0.2
	case rms < 0.03:
		energyScore = 0.6
	case rms < 0.15:
		energyScore = 1.0
	case rms < 0.25:
		energyScore = 0.7
	default:
		energyScore = 0.3
	}

	// Speech density: words per second. Natural conversational speech is
	// ~2-3 wps. Below 1.5 wps means long pauses or drawn-out words.
	// Above 3.5 wps means fast/rushed speech.
	words := strings.Fields(text)
	wps := float64(len(words)) / dur
	var densityScore float64
	switch {
	case wps < 0.5:
		densityScore = 0 // barely speaking
	case wps < 1.0:
		densityScore = 0.2 // very slow, lots of pauses/drawling
	case wps < 1.5:
		densityScore = 0.5 // slow
	case wps < 2.0:
		densityScore = 0.8 // slightly slow but ok
	case wps <= 3.5:
		densityScore = 1.0 // natural conversational pace
	case wps <= 4.5:
		densityScore = 0.7 // fast
	default:
		densityScore = 0.4 // too fast, probably garbled
	}

	// Duration score: prefer longer clips (more voice info).
	durScore := 1.0 - math.Abs(dur-idealDur)/idealDur
	if durScore < 0 {
		durScore = 0
	}

	consistency := energyConsistency(samples, sampleRate48k)
	textScore := textQuality(text)

	// Speech density is the strongest signal for "clear, natural speech".
	score := densityScore*0.35 + energyScore*0.20 + consistency*0.15 + durScore*0.15 + textScore*0.15
	return score, rms
}

// rmsEnergy computes the root-mean-square energy of the samples.
func rmsEnergy(samples []float32) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(len(samples)))
}

// energyConsistency measures how steady the volume is across the clip.
func energyConsistency(samples []float32, sampleRate int) float64 {
	windowSize := sampleRate / 4 // 250ms windows
	if windowSize == 0 || len(samples) < windowSize*2 {
		return 0.5
	}

	var windowRMS []float64
	for i := 0; i+windowSize <= len(samples); i += windowSize {
		windowRMS = append(windowRMS, rmsEnergy(samples[i:i+windowSize]))
	}
	if len(windowRMS) < 2 {
		return 0.5
	}

	// Filter out near-silent windows (pauses between words are normal).
	var active []float64
	for _, r := range windowRMS {
		if r > 0.005 {
			active = append(active, r)
		}
	}
	if len(active) < 2 {
		return 0.3
	}

	// Coefficient of variation (lower = more consistent).
	var mean float64
	for _, r := range active {
		mean += r
	}
	mean /= float64(len(active))

	var variance float64
	for _, r := range active {
		d := r - mean
		variance += d * d
	}
	cv := math.Sqrt(variance/float64(len(active))) / mean

	if cv < 0.3 {
		return 1.0
	}
	if cv > 1.0 {
		return 0.1
	}
	return 1.0 - (cv-0.3)/0.7*0.9
}

// textQuality scores transcript text for suitability as reference text.
func textQuality(text string) float64 {
	words := strings.Fields(text)
	if len(words) == 0 {
		return 0
	}

	// Prefer clips with more words (more text = better alignment signal).
	wordScore := float64(len(words)) / 15.0
	if wordScore > 1.0 {
		wordScore = 1.0
	}

	// Penalize filler-heavy text.
	fillers := 0
	for _, w := range words {
		lower := strings.ToLower(w)
		switch lower {
		case "um", "uh", "like", "hmm", "hm", "ah", "er", "mm", "yeah", "ugh":
			fillers++
		}
	}
	fillerRatio := float64(fillers) / float64(len(words))
	fillerPenalty := 1.0 - fillerRatio*3
	if fillerPenalty < 0 {
		fillerPenalty = 0
	}

	// Prefer text that looks like complete sentences.
	sentenceBonus := 0.0
	trimmed := strings.TrimSpace(text)
	if strings.HasSuffix(trimmed, ".") || strings.HasSuffix(trimmed, "!") || strings.HasSuffix(trimmed, "?") {
		sentenceBonus = 0.3
	}

	return wordScore*0.4 + fillerPenalty*0.4 + sentenceBonus*0.2
}

// cleanReferenceText trims the text to start and end on clean word/sentence
// boundaries. Removes leading lowercase fragments (likely mid-sentence) and
// ensures the text ends on a complete word.
func cleanReferenceText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	// Trim leading word if it starts lowercase and the text doesn't start
	// with "I" — likely a fragment from mid-sentence.
	first := words[0]
	if len(first) > 0 && first[0] >= 'a' && first[0] <= 'z' && first != "I" {
		// Check if this looks like a sentence fragment (no capital = mid-sentence).
		// Skip up to 3 lowercase words to find a sentence start.
		trimmed := false
		for i := 1; i < len(words) && i <= 3; i++ {
			w := words[i]
			if len(w) > 0 && ((w[0] >= 'A' && w[0] <= 'Z') || w == "I") {
				words = words[i:]
				trimmed = true
				break
			}
		}
		if !trimmed && len(words) > 1 {
			// No capital found nearby — just drop the first word.
			words = words[1:]
		}
	}

	if len(words) == 0 {
		return ""
	}

	return strings.Join(words, " ")
}

// normalizeRMS scales samples so their RMS matches the target level.
// This ensures consistent input volume regardless of recording conditions.
func normalizeRMS(samples []float32, targetRMS float64) {
	if len(samples) == 0 {
		return
	}
	currentRMS := rmsEnergy(samples)
	if currentRMS < 0.001 {
		return // near-silence, don't amplify noise
	}
	gain := float32(targetRMS / currentRMS)
	// Limit gain to avoid amplifying noise too much.
	if gain > 10.0 {
		gain = 10.0
	}
	for i := range samples {
		s := samples[i] * gain
		if s > 1.0 {
			s = 1.0
		} else if s < -1.0 {
			s = -1.0
		}
		samples[i] = s
	}
}

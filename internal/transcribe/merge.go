package transcribe

import (
	"fmt"
	"sort"
	"strings"
)

// UserSegment is a Segment attributed to a specific user/character.
type UserSegment struct {
	UserID        string
	CharacterName string // resolved from mapping, fallback to UserID
	Segment
}

// MergeTranscripts takes per-user segments, character name mappings, and
// per-user join offsets (seconds from session start to first audio). It adjusts
// each user's segment timestamps by their join offset, merges all segments
// chronologically (sorted by StartTime), and resolves UserID -> CharacterName.
func MergeTranscripts(userSegments map[string][]Segment, characterNames map[string]string, joinOffsets map[string]float64) []UserSegment {
	var merged []UserSegment

	for userID, segments := range userSegments {
		name := characterNames[userID] // empty string if no mapping
		offset := joinOffsets[userID]  // 0 if not present

		for _, seg := range segments {
			adjusted := seg
			adjusted.StartTime += offset
			adjusted.EndTime += offset
			merged = append(merged, UserSegment{
				UserID:        userID,
				CharacterName: name,
				Segment:       adjusted,
			})
		}
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].StartTime < merged[j].StartTime
	})

	return merged
}

// FormatTranscript renders merged segments as human-readable text.
//
// Each line has the form:
//
//	[HH:MM:SS] CharacterName: Transcript text here.
func FormatTranscript(segments []UserSegment) string {
	var b strings.Builder
	for _, seg := range segments {
		name := seg.CharacterName
		if name == "" {
			name = seg.UserID
		}
		b.WriteString(fmt.Sprintf("[%s] %s: %s\n", formatSeconds(seg.StartTime), name, seg.Text))
	}
	return b.String()
}

// TelegramEntry represents a Telegram message to be interleaved into a transcript.
type TelegramEntry struct {
	ElapsedSecs float64 // seconds since session start
	SenderName  string
	Text        string
}

// FormatTranscriptWithTelegram renders merged voice segments interleaved with
// Telegram messages, sorted chronologically by timestamp.
func FormatTranscriptWithTelegram(segments []UserSegment, telegramMsgs []TelegramEntry) string {
	// Build a unified timeline of tagged entries.
	type entry struct {
		time float64
		line string
	}

	var entries []entry
	for _, seg := range segments {
		name := seg.CharacterName
		if name == "" {
			name = seg.UserID
		}
		entries = append(entries, entry{
			time: seg.StartTime,
			line: fmt.Sprintf("[%s] %s: %s", formatSeconds(seg.StartTime), name, seg.Text),
		})
	}
	for _, tm := range telegramMsgs {
		entries = append(entries, entry{
			time: tm.ElapsedSecs,
			line: fmt.Sprintf("[%s] [%s via Telegram]: %s", formatSeconds(tm.ElapsedSecs), tm.SenderName, tm.Text),
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].time < entries[j].time
	})

	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.line)
		b.WriteByte('\n')
	}
	return b.String()
}

// formatSeconds converts a float64 seconds value to "HH:MM:SS".
func formatSeconds(secs float64) string {
	total := int(secs)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

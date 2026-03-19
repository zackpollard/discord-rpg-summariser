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

// MergeTranscripts takes per-user segments and character name mappings, merges
// all segments chronologically (sorted by StartTime), and resolves
// UserID -> CharacterName using the provided map. If no mapping exists for a
// user, the character name falls back to "User-{userID}".
func MergeTranscripts(userSegments map[string][]Segment, characterNames map[string]string) []UserSegment {
	var merged []UserSegment

	for userID, segments := range userSegments {
		name := characterNames[userID] // empty string if no mapping

		for _, seg := range segments {
			merged = append(merged, UserSegment{
				UserID:        userID,
				CharacterName: name,
				Segment:       seg,
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

// formatSeconds converts a float64 seconds value to "HH:MM:SS".
func formatSeconds(secs float64) string {
	total := int(secs)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

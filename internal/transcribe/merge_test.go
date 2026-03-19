package transcribe

import (
	"strings"
	"testing"
)

func TestMergeTranscripts_SortsChronologically(t *testing.T) {
	userSegments := map[string][]Segment{
		"user-a": {
			{StartTime: 5.0, EndTime: 8.0, Text: "I cast detect magic on the door."},
			{StartTime: 20.0, EndTime: 25.0, Text: "I open the door carefully."},
		},
		"user-b": {
			{StartTime: 12.0, EndTime: 18.0, Text: "The door glows with a faint aura of enchantment."},
			{StartTime: 0.5, EndTime: 3.0, Text: "You approach a heavy iron door."},
		},
	}

	characterNames := map[string]string{
		"user-a": "Thordak",
		"user-b": "DM",
	}

	merged := MergeTranscripts(userSegments, characterNames)

	if len(merged) != 4 {
		t.Fatalf("expected 4 merged segments, got %d", len(merged))
	}

	// Verify chronological order.
	for i := 1; i < len(merged); i++ {
		if merged[i].StartTime < merged[i-1].StartTime {
			t.Errorf("segment %d (%.1fs) is before segment %d (%.1fs)",
				i, merged[i].StartTime, i-1, merged[i-1].StartTime)
		}
	}

	// First segment should be user-b's earliest (0.5s).
	if merged[0].CharacterName != "DM" {
		t.Errorf("expected first segment from DM, got %q", merged[0].CharacterName)
	}
	if merged[0].StartTime != 0.5 {
		t.Errorf("expected first segment at 0.5s, got %.1fs", merged[0].StartTime)
	}
}

func TestMergeTranscripts_CharacterNameFallback(t *testing.T) {
	userSegments := map[string][]Segment{
		"known-user":   {{StartTime: 1.0, EndTime: 2.0, Text: "Hello"}},
		"unknown-user": {{StartTime: 3.0, EndTime: 4.0, Text: "World"}},
	}

	characterNames := map[string]string{
		"known-user": "Gandalf",
	}

	merged := MergeTranscripts(userSegments, characterNames)

	if len(merged) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(merged))
	}

	if merged[0].CharacterName != "Gandalf" {
		t.Errorf("expected known user to resolve to Gandalf, got %q", merged[0].CharacterName)
	}
	if merged[1].CharacterName != "" {
		t.Errorf("expected unknown user to have empty CharacterName, got %q", merged[1].CharacterName)
	}
}

func TestMergeTranscripts_EmptyInput(t *testing.T) {
	merged := MergeTranscripts(nil, nil)
	if len(merged) != 0 {
		t.Fatalf("expected 0 segments for nil input, got %d", len(merged))
	}

	merged = MergeTranscripts(map[string][]Segment{}, map[string]string{})
	if len(merged) != 0 {
		t.Fatalf("expected 0 segments for empty input, got %d", len(merged))
	}
}

func TestFormatTranscript(t *testing.T) {
	segments := []UserSegment{
		{
			UserID:        "user-a",
			CharacterName: "Thordak",
			Segment:       Segment{StartTime: 5.0, EndTime: 8.0, Text: "I cast detect magic on the door."},
		},
		{
			UserID:        "user-b",
			CharacterName: "DM",
			Segment:       Segment{StartTime: 12.0, EndTime: 18.0, Text: "The door glows with a faint aura of enchantment."},
		},
	}

	output := FormatTranscript(segments)

	expected := "[00:00:05] Thordak: I cast detect magic on the door.\n" +
		"[00:00:12] DM: The door glows with a faint aura of enchantment.\n"

	if output != expected {
		t.Errorf("format mismatch:\ngot:\n%s\nexpected:\n%s", output, expected)
	}
}

func TestFormatTranscript_LargeTimestamp(t *testing.T) {
	segments := []UserSegment{
		{
			CharacterName: "Bard",
			Segment:       Segment{StartTime: 3723.0, EndTime: 3730.0, Text: "We have been travelling for an hour."},
		},
	}

	output := FormatTranscript(segments)

	if !strings.Contains(output, "[01:02:03]") {
		t.Errorf("expected timestamp [01:02:03], got %q", output)
	}
}

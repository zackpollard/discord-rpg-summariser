package bot

import (
	"strings"
	"testing"

	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/transcribe"
)

type segEntry struct {
	userID, charName, text string
	start, end             float64
}

func makeSegments(entries ...segEntry) []transcribe.UserSegment {
	segs := make([]transcribe.UserSegment, len(entries))
	for i, e := range entries {
		segs[i] = transcribe.UserSegment{
			UserID:        e.userID,
			CharacterName: e.charName,
			Segment: transcribe.Segment{
				StartTime: e.start,
				EndTime:   e.end,
				Text:      e.text,
			},
		}
	}
	return segs
}

func TestBuildAnnotatedTranscript_FilterTableTalk(t *testing.T) {
	segs := makeSegments(
		segEntry{"u1", "Alice", "The dragon attacks!", 0, 5},
		segEntry{"u2", "Bob", "Anyone want pizza?", 5, 10},
		segEntry{"u1", "Alice", "I cast fireball.", 10, 15},
	)

	annotations := map[int64]*storage.TranscriptAnnotation{
		1: {SegmentID: 1, Classification: "narrative"},
		2: {SegmentID: 2, Classification: "table_talk"},
		3: {SegmentID: 3, Classification: "narrative"},
	}

	result := buildAnnotatedTranscript(segs, annotations, "DM")

	if !strings.Contains(result, "[TABLE TALK]") {
		t.Error("table_talk segment should be marked with [TABLE TALK]")
	}
	if !strings.Contains(result, "pizza") {
		t.Error("table_talk segment should still be present (marked, not removed)")
	}
	if !strings.Contains(result, "dragon") {
		t.Error("narrative segment should be present")
	}
	if !strings.Contains(result, "fireball") {
		t.Error("narrative segment should be present")
	}
}

func TestBuildAnnotatedTranscript_CorrectedText(t *testing.T) {
	segs := makeSegments(
		segEntry{"u1", "Alice", "I cassed fire ball", 0, 5},
	)

	corrected := "I cast fireball"
	annotations := map[int64]*storage.TranscriptAnnotation{
		1: {SegmentID: 1, Classification: "narrative", CorrectedText: &corrected},
	}

	result := buildAnnotatedTranscript(segs, annotations, "DM")

	if strings.Contains(result, "cassed") {
		t.Error("original text should be replaced by corrected text")
	}
	if !strings.Contains(result, "I cast fireball") {
		t.Errorf("corrected text should appear in output, got: %s", result)
	}
}

func TestBuildAnnotatedTranscript_SceneBoundaries(t *testing.T) {
	segs := makeSegments(
		segEntry{"u1", "Alice", "We enter the dungeon.", 0, 5},
		segEntry{"u1", "Alice", "The treasure is here.", 10, 15},
	)

	scene1 := "dungeon entrance"
	scene2 := "treasure room"
	annotations := map[int64]*storage.TranscriptAnnotation{
		1: {SegmentID: 1, Classification: "narrative", Scene: &scene1},
		2: {SegmentID: 2, Classification: "narrative", Scene: &scene2},
	}

	result := buildAnnotatedTranscript(segs, annotations, "DM")

	if !strings.Contains(result, "--- Dungeon entrance ---") {
		t.Errorf("expected scene boundary marker, got: %s", result)
	}
	if !strings.Contains(result, "--- Treasure room ---") {
		t.Errorf("expected second scene boundary marker, got: %s", result)
	}
}

func TestBuildAnnotatedTranscript_NPCVoice(t *testing.T) {
	segs := makeSegments(
		segEntry{"dm", "DM", "Welcome to my shop, adventurers.", 0, 5},
	)

	npc := "Shopkeeper"
	annotations := map[int64]*storage.TranscriptAnnotation{
		1: {SegmentID: 1, Classification: "narrative", NPCVoice: &npc},
	}

	result := buildAnnotatedTranscript(segs, annotations, "DM")

	if !strings.Contains(result, "DM (as Shopkeeper)") {
		t.Errorf("expected NPC voice label, got: %s", result)
	}
}

func TestBuildAnnotatedTranscript_MergeSegments(t *testing.T) {
	segs := makeSegments(
		segEntry{"u1", "Alice", "I want to", 0, 3},
		segEntry{"u1", "Alice", "cast fireball.", 3, 6},
	)

	annotations := map[int64]*storage.TranscriptAnnotation{
		1: {SegmentID: 1, Classification: "narrative", MergeWithNext: true},
		2: {SegmentID: 2, Classification: "narrative"},
	}

	result := buildAnnotatedTranscript(segs, annotations, "DM")

	if !strings.Contains(result, "I want to cast fireball.") {
		t.Errorf("merged segments should form one line, got: %s", result)
	}

	// Should only appear once as a single line, not two separate lines.
	lines := strings.Split(strings.TrimSpace(result), "\n")
	nonEmpty := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty++
		}
	}
	if nonEmpty != 1 {
		t.Errorf("expected 1 output line from merged segments, got %d: %s", nonEmpty, result)
	}
}

func TestBuildAnnotatedTranscript_NoAnnotations(t *testing.T) {
	segs := makeSegments(
		segEntry{"u1", "Alice", "Hello there.", 0, 5},
		segEntry{"u2", "Bob", "General Kenobi.", 5, 10},
	)

	result := buildAnnotatedTranscript(segs, nil, "DM")

	if !strings.Contains(result, "Alice: Hello there.") {
		t.Errorf("expected normal transcript line, got: %s", result)
	}
	if !strings.Contains(result, "Bob: General Kenobi.") {
		t.Errorf("expected normal transcript line, got: %s", result)
	}
}

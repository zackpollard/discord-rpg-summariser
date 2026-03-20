package embed

import (
	"strings"
	"testing"
)

func TestChunkTranscriptSegments_Empty(t *testing.T) {
	chunks := ChunkTranscriptSegments(nil, 1, nil)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(chunks))
	}
}

func TestChunkTranscriptSegments_SingleChunk(t *testing.T) {
	segments := []TranscriptSegment{
		{UserID: "user1", StartTime: 0, EndTime: 5, Text: "Hello there."},
		{UserID: "user2", StartTime: 5, EndTime: 10, Text: "General Kenobi!"},
	}
	charNames := map[string]string{
		"user1": "Obi-Wan",
		"user2": "Grievous",
	}

	chunks := ChunkTranscriptSegments(segments, 42, charNames)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	c := chunks[0]
	if c.DocType != "transcript_chunk" {
		t.Errorf("expected doc_type transcript_chunk, got %q", c.DocType)
	}
	if c.SessionID != 42 {
		t.Errorf("expected session_id 42, got %d", c.SessionID)
	}
	if !strings.Contains(c.Content, "Obi-Wan") {
		t.Error("chunk should contain character name Obi-Wan")
	}
	if !strings.Contains(c.Content, "Grievous") {
		t.Error("chunk should contain character name Grievous")
	}
	if !strings.Contains(c.Content, "Hello there.") {
		t.Error("chunk should contain segment text")
	}
}

func TestChunkTranscriptSegments_MultipleChunks(t *testing.T) {
	// Create enough segments to exceed maxChunkChars.
	var segments []TranscriptSegment
	longText := strings.Repeat("word ", 100) // ~500 chars each
	for i := 0; i < 10; i++ {
		segments = append(segments, TranscriptSegment{
			UserID:    "user1",
			StartTime: float64(i * 30),
			EndTime:   float64(i*30 + 25),
			Text:      longText,
		})
	}

	chunks := ChunkTranscriptSegments(segments, 1, map[string]string{"user1": "Speaker"})
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}

	// Verify no chunk exceeds the limit by too much (may exceed slightly
	// because we finish the current segment).
	for i, c := range chunks {
		if len(c.Content) > maxChunkChars+600 { // generous margin for last line
			t.Errorf("chunk %d too large: %d chars", i, len(c.Content))
		}
	}
}

func TestChunkTranscriptSegments_UnknownUser(t *testing.T) {
	segments := []TranscriptSegment{
		{UserID: "unknown123", StartTime: 0, EndTime: 5, Text: "I have no name."},
	}

	chunks := ChunkTranscriptSegments(segments, 1, nil)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Content, "unknown123") {
		t.Error("chunk should fall back to user ID when no character name exists")
	}
}

func TestBuildEntityText(t *testing.T) {
	text := BuildEntityText("Strahd", "npc", "Vampire lord of Barovia", []string{
		"Appeared at the tavern",
		"Threatened the party",
	})

	if !strings.Contains(text, "Strahd (npc)") {
		t.Error("should contain name and type")
	}
	if !strings.Contains(text, "Vampire lord") {
		t.Error("should contain description")
	}
	if !strings.Contains(text, "- Appeared at the tavern") {
		t.Error("should contain notes")
	}
}

func TestBuildEntityText_NoDescription(t *testing.T) {
	text := BuildEntityText("Unknown NPC", "npc", "", nil)
	if text != "Unknown NPC (npc)" {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestBuildQuestText(t *testing.T) {
	text := BuildQuestText("Find the Sword", "Retrieve the legendary blade.", "active", "Elder Sage", []string{
		"Learned the sword is in the dungeon",
	})

	if !strings.Contains(text, "Quest: Find the Sword [active]") {
		t.Error("should contain quest name and status")
	}
	if !strings.Contains(text, "(given by Elder Sage)") {
		t.Error("should contain giver")
	}
	if !strings.Contains(text, "Retrieve the legendary blade.") {
		t.Error("should contain description")
	}
	if !strings.Contains(text, "- Learned the sword is in the dungeon") {
		t.Error("should contain updates")
	}
}

func TestBuildQuestText_Minimal(t *testing.T) {
	text := BuildQuestText("Simple Quest", "", "active", "", nil)
	if text != "Quest: Simple Quest [active]" {
		t.Errorf("unexpected text: %q", text)
	}
}

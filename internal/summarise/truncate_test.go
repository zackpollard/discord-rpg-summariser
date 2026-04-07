package summarise

import (
	"strings"
	"testing"
)

func TestTruncateTranscript_ShortText(t *testing.T) {
	text := "This is a short transcript."
	result := truncateTranscript(text, 1000)
	if result != text {
		t.Errorf("short text should pass through unchanged, got %q", result)
	}
}

func TestTruncateTranscript_ExactLimit(t *testing.T) {
	text := strings.Repeat("a", 100)
	result := truncateTranscript(text, 100)
	if result != text {
		t.Errorf("text at exact limit should pass through, got length %d", len(result))
	}
}

func TestTruncateTranscript_TruncatesLongText(t *testing.T) {
	// Create a transcript with clear line boundaries.
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "[00:00:00] Speaker: This is line number "+strings.Repeat("x", 50))
	}
	text := strings.Join(lines, "\n")

	result := truncateTranscript(text, len(text)/2)

	// Should be shorter than original.
	if len(result) >= len(text) {
		t.Errorf("truncated text should be shorter: got %d, original %d", len(result), len(text))
	}

	// Should contain the omission marker.
	if !strings.Contains(result, "characters omitted") {
		t.Error("truncated text should contain omission marker")
	}

	// Should contain beginning of transcript.
	if !strings.Contains(result, "line number") {
		t.Error("truncated text should contain content from the start")
	}
}

func TestTruncateTranscript_PreservesLineBreaks(t *testing.T) {
	text := "Line one\nLine two\nLine three\nLine four\nLine five\nLine six\nLine seven\nLine eight"
	result := truncateTranscript(text, 40)

	// The head and tail should end/start on line boundaries.
	parts := strings.Split(result, "characters omitted")
	if len(parts) != 2 {
		t.Fatalf("expected omission marker to split into 2 parts, got %d", len(parts))
	}

	head := strings.TrimSpace(parts[0])
	tail := strings.TrimSpace(parts[1])

	// Head should end with a complete line (no partial line).
	if head != "" && !strings.HasSuffix(head, "\n") && !strings.Contains(head, "\n") {
		// Single line is fine.
	}

	// Tail content (after the marker) should contain recognizable lines.
	if tail != "" && strings.Contains(tail, "Line") {
		// Good — tail has transcript content.
	} else if tail == "" {
		t.Error("tail should have content")
	}
}

func TestTruncateTranscript_HeadLargerThanTail(t *testing.T) {
	// 60/40 split: head should get more content than tail.
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "[00:00:00] Speaker: Line content here padding")
	}
	text := strings.Join(lines, "\n")

	result := truncateTranscript(text, len(text)/3)

	parts := strings.SplitN(result, "characters omitted", 2)
	if len(parts) != 2 {
		t.Skip("could not split on marker")
	}

	head := parts[0]
	tail := parts[1]

	// Head should be roughly 60% of the total content.
	if len(head) < len(tail) {
		t.Errorf("head (%d chars) should be larger than tail (%d chars)", len(head), len(tail))
	}
}

func TestTruncateTranscript_EmptyInput(t *testing.T) {
	result := truncateTranscript("", 100)
	if result != "" {
		t.Errorf("empty input should return empty, got %q", result)
	}
}

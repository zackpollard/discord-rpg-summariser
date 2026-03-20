package transcribe

import (
	"math"
	"testing"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// ---------------------------------------------------------------------------
// resultToSegments
// ---------------------------------------------------------------------------

func TestResultToSegments_EmptyText(t *testing.T) {
	result := &sherpa.OfflineRecognizerResult{Text: ""}
	got := resultToSegments(result, 0)
	if got != nil {
		t.Errorf("empty text: got %v, want nil", got)
	}
}

func TestResultToSegments_WhitespaceText(t *testing.T) {
	result := &sherpa.OfflineRecognizerResult{Text: "   \n\t  "}
	got := resultToSegments(result, 0)
	if got != nil {
		t.Errorf("whitespace-only text: got %v, want nil", got)
	}
}

func TestResultToSegments_NoTokens(t *testing.T) {
	// Text is present but no token-level timing.
	result := &sherpa.OfflineRecognizerResult{
		Text: "Hello world",
	}
	got := resultToSegments(result, 0)
	if len(got) != 1 {
		t.Fatalf("no tokens: expected 1 segment, got %d", len(got))
	}
	if got[0].Text != "Hello world" {
		t.Errorf("text: got %q, want %q", got[0].Text, "Hello world")
	}
	if got[0].StartTime != 0 || got[0].EndTime != 0 {
		t.Errorf("times should be 0 with no tokens: start=%f end=%f", got[0].StartTime, got[0].EndTime)
	}
}

func TestResultToSegments_NoTokensWithOffset(t *testing.T) {
	result := &sherpa.OfflineRecognizerResult{
		Text: "Hello world",
	}
	got := resultToSegments(result, 5.0)
	if len(got) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(got))
	}
	if got[0].StartTime != 5.0 {
		t.Errorf("start time with offset: got %f, want 5.0", got[0].StartTime)
	}
}

func TestResultToSegments_SingleSentence(t *testing.T) {
	result := &sherpa.OfflineRecognizerResult{
		Text:       "Hello world.",
		Tokens:     []string{"\u2581Hello", "\u2581world", "."},
		Timestamps: []float32{0.0, 0.5, 1.0},
		Durations:  []float32{0.4, 0.4, 0.1},
	}
	got := resultToSegments(result, 0)
	if len(got) != 1 {
		t.Fatalf("single sentence: expected 1 segment, got %d", len(got))
	}
	if got[0].Text != "Hello world." {
		t.Errorf("text: got %q, want %q", got[0].Text, "Hello world.")
	}
	// Start = timestamps[0] = 0.0, End = timestamps[2] + durations[2] = 1.1
	assertFloat(t, "start", got[0].StartTime, 0.0)
	assertFloat(t, "end", got[0].EndTime, 1.1)
}

func TestResultToSegments_MultipleSentences(t *testing.T) {
	result := &sherpa.OfflineRecognizerResult{
		Text: "Hello. World!",
		Tokens: []string{
			"\u2581Hello", ".",
			"\u2581World", "!",
		},
		Timestamps: []float32{0.0, 0.4, 1.0, 1.4},
		Durations:  []float32{0.3, 0.1, 0.3, 0.1},
	}
	got := resultToSegments(result, 0)
	if len(got) != 2 {
		t.Fatalf("two sentences: expected 2 segments, got %d", len(got))
	}
	if got[0].Text != "Hello." {
		t.Errorf("seg 0 text: got %q, want %q", got[0].Text, "Hello.")
	}
	if got[1].Text != "World!" {
		t.Errorf("seg 1 text: got %q, want %q", got[1].Text, "World!")
	}
	assertFloat(t, "seg1 start", got[1].StartTime, 1.0)
	assertFloat(t, "seg1 end", got[1].EndTime, 1.5)
}

func TestResultToSegments_TimeOffset(t *testing.T) {
	result := &sherpa.OfflineRecognizerResult{
		Text:       "Hello.",
		Tokens:     []string{"\u2581Hello", "."},
		Timestamps: []float32{0.0, 0.5},
		Durations:  []float32{0.4, 0.1},
	}
	got := resultToSegments(result, 10.0)
	if len(got) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(got))
	}
	assertFloat(t, "start with offset", got[0].StartTime, 10.0)
	assertFloat(t, "end with offset", got[0].EndTime, 10.6)
}

func TestResultToSegments_LongPauseSplits(t *testing.T) {
	// Tokens with a >2s gap between token 1 and token 2.
	result := &sherpa.OfflineRecognizerResult{
		Text: "Hello World",
		Tokens: []string{
			"\u2581Hello",
			"\u2581World",
		},
		Timestamps: []float32{0.0, 3.0},
		Durations:  []float32{0.3, 0.3},
	}
	// Gap: token0 ends at 0.0+0.3=0.3, token1 starts at 3.0 => gap = 2.7 > 2.0
	got := resultToSegments(result, 0)
	if len(got) != 2 {
		t.Fatalf("long pause split: expected 2 segments, got %d", len(got))
	}
	if got[0].Text != "Hello" {
		t.Errorf("seg 0 text: got %q, want %q", got[0].Text, "Hello")
	}
	if got[1].Text != "World" {
		t.Errorf("seg 1 text: got %q, want %q", got[1].Text, "World")
	}
}

func TestResultToSegments_BPEReplacement(t *testing.T) {
	// BPE word-boundary markers (U+2581) should be replaced with spaces.
	result := &sherpa.OfflineRecognizerResult{
		Text:       "The quick fox.",
		Tokens:     []string{"\u2581The", "\u2581quick", "\u2581fox", "."},
		Timestamps: []float32{0.0, 0.3, 0.6, 0.9},
		Durations:  []float32{0.2, 0.2, 0.2, 0.1},
	}
	got := resultToSegments(result, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(got))
	}
	if got[0].Text != "The quick fox." {
		t.Errorf("BPE replacement: got %q, want %q", got[0].Text, "The quick fox.")
	}
}

func TestResultToSegments_NoDurations(t *testing.T) {
	// Timestamps present but no durations.
	result := &sherpa.OfflineRecognizerResult{
		Text:       "Hello.",
		Tokens:     []string{"\u2581Hello", "."},
		Timestamps: []float32{0.0, 0.5},
	}
	got := resultToSegments(result, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(got))
	}
	// Without durations, end = timestamps[last] = 0.5 (no duration added).
	assertFloat(t, "end without durations", got[0].EndTime, 0.5)
}

func TestResultToSegments_QuestionMark(t *testing.T) {
	result := &sherpa.OfflineRecognizerResult{
		Text:       "Really? Yes.",
		Tokens:     []string{"\u2581Really", "?", "\u2581Yes", "."},
		Timestamps: []float32{0.0, 0.5, 1.0, 1.3},
		Durations:  []float32{0.4, 0.1, 0.2, 0.1},
	}
	got := resultToSegments(result, 0)
	if len(got) != 2 {
		t.Fatalf("question mark split: expected 2 segments, got %d", len(got))
	}
	if got[0].Text != "Really?" {
		t.Errorf("seg 0: got %q, want %q", got[0].Text, "Really?")
	}
	if got[1].Text != "Yes." {
		t.Errorf("seg 1: got %q, want %q", got[1].Text, "Yes.")
	}
}

// assertFloat checks that got is within epsilon of want.
func assertFloat(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-4 {
		t.Errorf("%s: got %f, want %f", name, got, want)
	}
}

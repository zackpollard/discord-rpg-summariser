package summarise

import (
	"testing"
)

func TestStripCodeFences_PureJSON(t *testing.T) {
	input := []byte(`{"key": "value"}`)
	got := string(StripCodeFences(input))
	want := `{"key": "value"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripCodeFences_CodeFence(t *testing.T) {
	input := []byte("```json\n{\"key\": \"value\"}\n```")
	got := string(StripCodeFences(input))
	want := `{"key": "value"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripCodeFences_TextBeforeCodeFence(t *testing.T) {
	input := []byte("Here is the result:\n\n```json\n{\"key\": \"value\"}\n```")
	got := string(StripCodeFences(input))
	want := `{"key": "value"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripCodeFences_TextBeforeRawJSON(t *testing.T) {
	input := []byte("Looking at the data...\n\n{\"key\": \"value\"}")
	got := string(StripCodeFences(input))
	want := `{"key": "value"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripCodeFences_JSONArray(t *testing.T) {
	input := []byte(`[{"a": 1}]`)
	got := string(StripCodeFences(input))
	want := `[{"a": 1}]`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripCodeFences_TextBeforeArray(t *testing.T) {
	input := []byte("Results:\n[1, 2, 3]")
	got := string(StripCodeFences(input))
	want := `[1, 2, 3]`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripCodeFences_EmptyInput(t *testing.T) {
	got := string(StripCodeFences([]byte("")))
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestStripCodeFences_NoJSON(t *testing.T) {
	input := []byte("This is plain text with no JSON at all.")
	got := string(StripCodeFences(input))
	want := "This is plain text with no JSON at all."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

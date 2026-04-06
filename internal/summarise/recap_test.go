package summarise

import (
	"strings"
	"testing"
)

func TestBuildRecapPrompt_Basic(t *testing.T) {
	summaries := []string{"The party arrived at the village and met the blacksmith."}

	prompt := BuildRecapPrompt(summaries, "")

	if !strings.Contains(prompt, summaries[0]) {
		t.Error("prompt should contain the session summary")
	}
	if !strings.Contains(prompt, "Session 1") {
		t.Error("prompt should label session numbers")
	}
	if !strings.Contains(prompt, "Story So Far") {
		t.Error("prompt should mention 'Story So Far'")
	}
	if !strings.Contains(prompt, "valid JSON") {
		t.Error("prompt should instruct to return valid JSON")
	}
	if !strings.Contains(prompt, `"recap"`) {
		t.Error("prompt should contain JSON field instruction for recap")
	}
	if !strings.Contains(prompt, "past tense") {
		t.Error("prompt should instruct to write in past tense")
	}
	if !strings.Contains(prompt, "character names") {
		t.Error("prompt should instruct to use character names")
	}
}

func TestBuildRecapPrompt_MultipleSessions(t *testing.T) {
	summaries := []string{
		"Session one: the party entered the dungeon.",
		"Session two: the party defeated the dragon.",
		"Session three: the party celebrated in town.",
	}

	prompt := BuildRecapPrompt(summaries, "")

	// All summaries should appear in the prompt.
	for i, s := range summaries {
		if !strings.Contains(prompt, s) {
			t.Errorf("prompt should contain session summary %d", i+1)
		}
	}

	// Session labels should be in order.
	if !strings.Contains(prompt, "Session 1") {
		t.Error("prompt should contain Session 1 label")
	}
	if !strings.Contains(prompt, "Session 2") {
		t.Error("prompt should contain Session 2 label")
	}
	if !strings.Contains(prompt, "Session 3") {
		t.Error("prompt should contain Session 3 label")
	}

	// Session 1 should appear before Session 2 in the prompt.
	idx1 := strings.Index(prompt, "Session 1")
	idx2 := strings.Index(prompt, "Session 2")
	idx3 := strings.Index(prompt, "Session 3")
	if idx1 >= idx2 || idx2 >= idx3 {
		t.Error("session labels should appear in chronological order")
	}
}

func TestBuildRecapPrompt_WithDMName(t *testing.T) {
	prompt := BuildRecapPrompt([]string{"A session happened."}, "Matt")

	if !strings.Contains(prompt, "The Dungeon Master is: Matt") {
		t.Error("prompt should identify the DM by name")
	}
}

func TestBuildRecapPrompt_NoDMName(t *testing.T) {
	prompt := BuildRecapPrompt([]string{"A session happened."}, "")

	if strings.Contains(prompt, "Dungeon Master is:") {
		t.Error("prompt should not contain DM section when dmName is empty")
	}
}

func TestBuildRecapPrompt_PartialLastN(t *testing.T) {
	summaries := []string{
		"Session four: the party found the lost temple.",
		"Session five: the party confronted the lich.",
	}

	prompt := BuildRecapPrompt(summaries, "Matt", RecapPromptOptions{LastN: 2})

	if !strings.Contains(prompt, "most recent 2 sessions") {
		t.Error("partial prompt should mention 'most recent 2 sessions'")
	}
	if !strings.Contains(prompt, "recent events") {
		t.Error("partial prompt should mention 'recent events'")
	}
	// Should NOT contain the full "Story So Far" preamble.
	if strings.Contains(prompt, "Story So Far") {
		t.Error("partial prompt should not contain 'Story So Far'")
	}
	// All summaries should still appear.
	for i, s := range summaries {
		if !strings.Contains(prompt, s) {
			t.Errorf("prompt should contain session summary %d", i+1)
		}
	}
	// DM name should still appear.
	if !strings.Contains(prompt, "The Dungeon Master is: Matt") {
		t.Error("prompt should contain DM name")
	}
}

func TestBuildRecapPrompt_ZeroLastN(t *testing.T) {
	// lastN=0 should behave the same as the default (full recap).
	prompt := BuildRecapPrompt([]string{"A session happened."}, "", RecapPromptOptions{LastN: 0})

	if !strings.Contains(prompt, "Story So Far") {
		t.Error("prompt with lastN=0 should contain 'Story So Far'")
	}
	if strings.Contains(prompt, "most recent") {
		t.Error("prompt with lastN=0 should not contain 'most recent'")
	}
}

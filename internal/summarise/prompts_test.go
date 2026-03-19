package summarise

import (
	"strings"
	"testing"
)

func TestBuildPrompt_WithoutPreviousSummary(t *testing.T) {
	transcript := "[00:00:05] Thordak: I open the chest.\n[00:00:12] DM: Inside you find a glowing orb."

	prompt := BuildPrompt(transcript, "", "")

	if !strings.Contains(prompt, "Dungeons & Dragons 5th Edition") {
		t.Error("prompt should mention D&D 5e")
	}
	if strings.Contains(prompt, "Previously:") {
		t.Error("prompt should not contain 'Previously:' when no previous summary is given")
	}
	if !strings.Contains(prompt, transcript) {
		t.Error("prompt should contain the transcript")
	}
	if !strings.Contains(prompt, `"summary"`) {
		t.Error("prompt should contain JSON field instruction for summary")
	}
	if !strings.Contains(prompt, `"key_events"`) {
		t.Error("prompt should contain JSON field instruction for key_events")
	}
	if !strings.Contains(prompt, `"npcs"`) {
		t.Error("prompt should contain JSON field instruction for npcs")
	}
	if !strings.Contains(prompt, `"places"`) {
		t.Error("prompt should contain JSON field instruction for places")
	}
	if !strings.Contains(prompt, "character names") {
		t.Error("prompt should instruct to use character names")
	}
	if !strings.Contains(prompt, "combat") {
		t.Error("prompt should mention combat encounters")
	}
	if !strings.Contains(prompt, "lore") {
		t.Error("prompt should mention lore")
	}
}

func TestBuildPrompt_WithPreviousSummary(t *testing.T) {
	transcript := "[00:00:05] Thordak: I open the chest."
	previousSummary := "The party defeated a band of goblins and entered the dungeon."

	prompt := BuildPrompt(transcript, previousSummary, "")

	if !strings.Contains(prompt, "Previously:") {
		t.Error("prompt should contain 'Previously:' when previous summary is given")
	}
	if !strings.Contains(prompt, previousSummary) {
		t.Error("prompt should contain the previous summary text")
	}
	if !strings.Contains(prompt, transcript) {
		t.Error("prompt should still contain the transcript")
	}
}

func TestBuildPrompt_JSONInstruction(t *testing.T) {
	prompt := BuildPrompt("some transcript", "", "")

	if !strings.Contains(prompt, "valid JSON") {
		t.Error("prompt should instruct the model to return valid JSON")
	}
}

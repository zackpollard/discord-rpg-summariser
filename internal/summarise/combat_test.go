package summarise

import (
	"strings"
	"testing"
)

func TestBuildCombatExtractionPrompt_Basic(t *testing.T) {
	transcript := "[00:02:00] DM: Roll initiative! The goblins attack!\n[00:02:30] Thordak: I swing my greatsword at the goblin."
	summary := "The party was ambushed by goblins while crossing the bridge."

	prompt := BuildCombatExtractionPrompt(transcript, summary, "", nil)

	if !strings.Contains(prompt, transcript) {
		t.Error("prompt should contain the transcript")
	}
	if !strings.Contains(prompt, summary) {
		t.Error("prompt should contain the summary")
	}
	if !strings.Contains(prompt, "combat") {
		t.Error("prompt should contain combat extraction instructions")
	}
	if !strings.Contains(prompt, "valid JSON") {
		t.Error("prompt should instruct to return valid JSON")
	}
	if !strings.Contains(prompt, "encounters") {
		t.Error("prompt should mention encounters in the JSON schema")
	}
	if !strings.Contains(prompt, "actions") {
		t.Error("prompt should mention actions in the JSON schema")
	}
	if !strings.Contains(prompt, "action_type") {
		t.Error("prompt should mention action_type field")
	}
	if !strings.Contains(prompt, "attack") {
		t.Error("prompt should mention attack action type")
	}
	if !strings.Contains(prompt, "spell") {
		t.Error("prompt should mention spell action type")
	}
	if !strings.Contains(prompt, "heal") {
		t.Error("prompt should mention heal action type")
	}
}

func TestBuildCombatExtractionPrompt_WithDMName(t *testing.T) {
	prompt := BuildCombatExtractionPrompt("transcript", "summary", "Matt", nil)

	if !strings.Contains(prompt, "The Dungeon Master is: Matt") {
		t.Error("prompt should identify the DM by name")
	}
	if !strings.Contains(prompt, "Combat descriptions") {
		t.Error("prompt should mention combat descriptions come from the DM")
	}
}

func TestBuildCombatExtractionPrompt_NoDMName(t *testing.T) {
	prompt := BuildCombatExtractionPrompt("transcript", "summary", "", nil)

	if strings.Contains(prompt, "Dungeon Master is:") {
		t.Error("prompt should not contain DM section when dmName is empty")
	}
}

func TestBuildCombatExtractionPrompt_WithPlayerCharacters(t *testing.T) {
	pcs := []string{"Thordak", "Elara", "Grimjaw"}
	prompt := BuildCombatExtractionPrompt("transcript", "summary", "", pcs)

	if !strings.Contains(prompt, "PLAYER CHARACTERS") {
		t.Error("prompt should contain player characters section")
	}
	for _, name := range pcs {
		if !strings.Contains(prompt, name) {
			t.Errorf("prompt should contain player character %q", name)
		}
	}
}

func TestBuildCombatExtractionPrompt_NoCombatInstruction(t *testing.T) {
	prompt := BuildCombatExtractionPrompt("transcript", "summary", "", nil)

	if !strings.Contains(prompt, "empty encounters array") {
		t.Error("prompt should instruct to return empty array when no combat")
	}
}

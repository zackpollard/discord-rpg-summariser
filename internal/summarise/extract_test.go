package summarise

import (
	"strings"
	"testing"
)

func TestBuildExtractionPrompt_Basic(t *testing.T) {
	transcript := "[00:00:05] Thordak: I open the chest.\n[00:00:12] DM: Inside you find a glowing orb."
	summary := "The party found a glowing orb in the dungeon."

	prompt := BuildExtractionPrompt(transcript, summary, nil, "", nil)

	if !strings.Contains(prompt, transcript) {
		t.Error("prompt should contain the transcript")
	}
	if !strings.Contains(prompt, summary) {
		t.Error("prompt should contain the summary")
	}
	if !strings.Contains(prompt, "Guidelines:") {
		t.Error("prompt should contain guidelines section")
	}
	if !strings.Contains(prompt, "valid JSON") {
		t.Error("prompt should instruct to return valid JSON")
	}
	if !strings.Contains(prompt, "entities") {
		t.Error("prompt should mention entities in the JSON schema")
	}
	if !strings.Contains(prompt, "relationships") {
		t.Error("prompt should mention relationships in the JSON schema")
	}
	if !strings.Contains(prompt, "character names") {
		t.Error("prompt should instruct to use character names")
	}
}

func TestBuildExtractionPrompt_WithExistingEntities(t *testing.T) {
	entities := []string{"Strahd von Zarovich", "Barovia", "Ireena Kolyana"}

	prompt := BuildExtractionPrompt("transcript", "summary", entities, "", nil)

	for _, name := range entities {
		if !strings.Contains(prompt, name) {
			t.Errorf("prompt should contain existing entity %q", name)
		}
	}
	if !strings.Contains(prompt, "already exist in the knowledge base") {
		t.Error("prompt should mention existing entities context")
	}
}

func TestBuildExtractionPrompt_WithDMName(t *testing.T) {
	prompt := BuildExtractionPrompt("transcript", "summary", nil, "Matt", nil)

	if !strings.Contains(prompt, "The Dungeon Master is: Matt") {
		t.Error("prompt should identify the DM by name")
	}
	if !strings.Contains(prompt, "Matt") {
		t.Error("prompt should contain the DM name")
	}
	if !strings.Contains(prompt, "narration") {
		t.Error("prompt should describe DM's role as narrator")
	}
	if !strings.Contains(prompt, "Do NOT create an entity for the DM") {
		t.Error("prompt should instruct not to create an entity for the DM")
	}
}

func TestBuildExtractionPrompt_WithPlayerCharacters(t *testing.T) {
	pcs := []string{"Thordak", "Elara", "Grimjaw"}
	prompt := BuildExtractionPrompt("transcript", "summary", nil, "", pcs)

	if !strings.Contains(prompt, "PLAYER CHARACTERS") {
		t.Error("prompt should contain player characters section")
	}
	for _, name := range pcs {
		if !strings.Contains(prompt, name) {
			t.Errorf("prompt should contain player character %q", name)
		}
	}
	if !strings.Contains(prompt, "already exist as type 'pc'") {
		t.Error("prompt should mention PCs already exist as type 'pc'")
	}
	if !strings.Contains(prompt, "DO include relationships where a player character is the source or target") {
		t.Error("prompt should instruct to include PC relationships")
	}
	if !strings.Contains(prompt, "exact character names") {
		t.Error("prompt should instruct to use exact character names for relationship source/target")
	}
	if !strings.Contains(prompt, "Do NOT extract player characters as entities") {
		t.Error("prompt should instruct not to extract PCs as entities")
	}
}

func TestBuildExtractionPrompt_IncludesStatusInstructions(t *testing.T) {
	prompt := BuildExtractionPrompt("transcript", "summary", nil, "", nil)

	if !strings.Contains(prompt, "status") {
		t.Error("prompt should contain status field instructions")
	}
	if !strings.Contains(prompt, "'alive'") {
		t.Error("prompt should mention 'alive' as a valid status")
	}
	if !strings.Contains(prompt, "'dead'") {
		t.Error("prompt should mention 'dead' as a valid status")
	}
	if !strings.Contains(prompt, "'unknown'") {
		t.Error("prompt should mention 'unknown' as a valid status")
	}
	if !strings.Contains(prompt, "cause_of_death") {
		t.Error("prompt should mention cause_of_death in the JSON schema")
	}
	if !strings.Contains(prompt, "For each NPC, include their current status") {
		t.Error("prompt should instruct the LLM to include NPC status")
	}
}

func TestBuildExtractionPrompt_IncludesParentPlaceInstruction(t *testing.T) {
	prompt := BuildExtractionPrompt("transcript", "summary", nil, "", nil)

	if !strings.Contains(prompt, "parent_place") {
		t.Error("prompt should contain parent_place in the JSON schema")
	}
	if !strings.Contains(prompt, "located within another place") {
		t.Error("prompt should instruct the LLM to detect location containment")
	}
	if !strings.Contains(prompt, "set `parent_place` to the name of the containing place") {
		t.Error("prompt should instruct to set parent_place field")
	}
}

func TestBuildExtractionPrompt_NoDMName(t *testing.T) {
	prompt := BuildExtractionPrompt("transcript", "summary", nil, "", nil)

	if strings.Contains(prompt, "Dungeon Master is:") {
		t.Error("prompt should not contain DM section when dmName is empty")
	}
	if strings.Contains(prompt, "narration, NPC dialogue") {
		t.Error("prompt should not contain DM narration instructions when dmName is empty")
	}
}

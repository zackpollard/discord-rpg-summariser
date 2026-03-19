package summarise

import (
	"strings"
	"testing"
)

func TestBuildQuestExtractionPrompt_Basic(t *testing.T) {
	transcript := "[00:05:00] DM: The blacksmith asks you to retrieve his stolen hammer."
	summary := "The party received a quest from the village blacksmith."

	prompt := BuildQuestExtractionPrompt(transcript, summary, nil, "")

	if !strings.Contains(prompt, transcript) {
		t.Error("prompt should contain the transcript")
	}
	if !strings.Contains(prompt, summary) {
		t.Error("prompt should contain the summary")
	}
	if !strings.Contains(prompt, "quest") {
		t.Error("prompt should contain quest extraction instructions")
	}
	if !strings.Contains(prompt, "valid JSON") {
		t.Error("prompt should instruct to return valid JSON")
	}
	if !strings.Contains(prompt, "quests") {
		t.Error("prompt should mention quests in the JSON schema")
	}
	if !strings.Contains(prompt, "status") {
		t.Error("prompt should mention quest status field")
	}
	if !strings.Contains(prompt, "active") {
		t.Error("prompt should mention active status")
	}
	if !strings.Contains(prompt, "completed") {
		t.Error("prompt should mention completed status")
	}
	if !strings.Contains(prompt, "failed") {
		t.Error("prompt should mention failed status")
	}
}

func TestBuildQuestExtractionPrompt_WithExistingQuests(t *testing.T) {
	quests := []string{"Retrieve the Blacksmith's Hammer", "Escort the Merchant Caravan"}

	prompt := BuildQuestExtractionPrompt("transcript", "summary", quests, "")

	for _, name := range quests {
		if !strings.Contains(prompt, name) {
			t.Errorf("prompt should contain existing quest %q", name)
		}
	}
	if !strings.Contains(prompt, "already exist in the tracker") {
		t.Error("prompt should mention existing quests context")
	}
	if !strings.Contains(prompt, "do not create duplicates") {
		t.Error("prompt should warn against duplicates")
	}
}

func TestBuildQuestExtractionPrompt_WithDMName(t *testing.T) {
	prompt := BuildQuestExtractionPrompt("transcript", "summary", nil, "Matt")

	if !strings.Contains(prompt, "The Dungeon Master is: Matt") {
		t.Error("prompt should identify the DM by name")
	}
	if !strings.Contains(prompt, "Quests are typically given by NPCs voiced by the DM") {
		t.Error("prompt should mention quests come from DM-voiced NPCs")
	}
}

func TestBuildQuestExtractionPrompt_NoDMName(t *testing.T) {
	prompt := BuildQuestExtractionPrompt("transcript", "summary", nil, "")

	if strings.Contains(prompt, "Dungeon Master is:") {
		t.Error("prompt should not contain DM section when dmName is empty")
	}
}

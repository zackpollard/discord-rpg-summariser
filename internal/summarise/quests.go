package summarise

import (
	"context"
	"fmt"
	"strings"
)

// ExtractedQuest represents a single quest extracted from a session transcript.
type ExtractedQuest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Giver       string `json:"giver"`  // NPC who gave the quest
	Status      string `json:"status"` // active, completed, failed
	Update      string `json:"update"` // what happened this session
}

// QuestExtractionResult holds all quests extracted from a session.
type QuestExtractionResult struct {
	Quests []ExtractedQuest `json:"quests"`
}

// QuestExtractor produces structured quest data from a session transcript.
type QuestExtractor interface {
	ExtractQuests(ctx context.Context, transcript, summary string, existingQuests []string, dmName string) (*QuestExtractionResult, error)
}

// BuildQuestExtractionPrompt constructs the LLM prompt for extracting quests
// from a session transcript and its summary.
func BuildQuestExtractionPrompt(transcript, summary string, existingQuests []string, dmName string) string {
	var b strings.Builder

	b.WriteString("You are an expert quest tracker for tabletop RPG campaigns (Dungeons & Dragons 5th Edition).\n\n")

	if dmName != "" {
		b.WriteString("The Dungeon Master is: " + dmName + "\n")
		b.WriteString("Everything " + dmName + " says is narration, NPC dialogue, or world description — not a player character.\n")
		b.WriteString("Quests are typically given by NPCs voiced by the DM.\n\n")
	}

	b.WriteString("Below is a session transcript and its summary. ")
	b.WriteString("Analyse them carefully and extract all quests — both newly introduced and updates to existing ones.\n\n")

	b.WriteString("Guidelines:\n")
	b.WriteString("- Identify quests: tasks, missions, objectives, or goals given to the party.\n")
	b.WriteString("- Use concise, descriptive quest names.\n")
	b.WriteString("- For each quest, note the NPC who gave it (giver), a brief description, and what happened this session (update).\n")
	b.WriteString("- Set status to: active (ongoing), completed (finished this session), or failed (failed this session).\n")
	b.WriteString("- If a quest was not resolved this session, its status should be active.\n")
	b.WriteString("- Use character/NPC names, not player names.\n")

	if len(existingQuests) > 0 {
		b.WriteString("\nThe following quests already exist in the tracker. ")
		b.WriteString("Use these exact names when referring to them (do not create duplicates):\n")
		for _, name := range existingQuests {
			fmt.Fprintf(&b, "- %s\n", name)
		}
	}

	b.WriteString("\nReturn ONLY valid JSON with exactly these fields:\n")
	b.WriteString("{\n")
	b.WriteString("  \"quests\": [\n")
	b.WriteString("    {\"name\": \"Quest Name\", \"description\": \"What the quest is about.\", \"giver\": \"NPC Name\", \"status\": \"active\", \"update\": \"What happened this session.\"}\n")
	b.WriteString("  ]\n")
	b.WriteString("}\n\n")

	b.WriteString("Session Summary:\n")
	b.WriteString(summary)
	b.WriteString("\n\n---\n\n")

	b.WriteString("Transcript:\n")
	b.WriteString(transcript)

	return b.String()
}

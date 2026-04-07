package summarise

import (
	"context"
	"fmt"
	"strings"
)

// CombatExtractedAction represents a single action in a combat encounter.
type CombatExtractedAction struct {
	Actor      string   `json:"actor"`
	ActionType string   `json:"action_type"` // attack, spell, ability, heal, damage_taken, save, skill
	Target     string   `json:"target"`
	Detail     string   `json:"detail"`
	Damage     *int     `json:"damage"`
	Round      *int     `json:"round"`
	Timestamp  *float64 `json:"timestamp"`
}

// CombatExtractedEncounter represents a combat encounter extracted from a transcript.
type CombatExtractedEncounter struct {
	Name      string                  `json:"name"`
	StartTime float64                 `json:"start_time"`
	EndTime   float64                 `json:"end_time"`
	Summary   string                  `json:"summary"`
	Actions   []CombatExtractedAction `json:"actions"`
}

// CombatExtractionResult holds all combat encounters extracted from a session.
type CombatExtractionResult struct {
	Encounters []CombatExtractedEncounter `json:"encounters"`
}

// CombatExtractor produces structured combat encounter data from a session transcript.
type CombatExtractor interface {
	ExtractCombat(ctx context.Context, transcript, summary, dmName string, playerCharacters []string) (*CombatExtractionResult, error)
}

// BuildCombatExtractionPrompt constructs the LLM prompt for detecting and
// extracting combat encounters from a session transcript and its summary.
func BuildCombatExtractionPrompt(transcript, summary, dmName string, playerCharacters []string) string {
	var b strings.Builder

	b.WriteString("You are an expert combat analyst for tabletop RPG campaigns (Dungeons & Dragons 5th Edition).\n\n")

	if dmName != "" {
		b.WriteString("The Dungeon Master is: " + dmName + "\n")
		b.WriteString("Everything " + dmName + " says is narration, NPC dialogue, or world description.\n")
		b.WriteString("Combat descriptions, damage announcements, and enemy actions come from the DM.\n\n")
	}

	if len(playerCharacters) > 0 {
		b.WriteString("The following are PLAYER CHARACTERS in this session:\n")
		for _, name := range playerCharacters {
			fmt.Fprintf(&b, "- %s\n", name)
		}
		b.WriteByte('\n')
	}

	b.WriteString("Below is a session transcript and its summary. ")
	b.WriteString("Analyse them carefully and identify all combat encounters.\n\n")

	b.WriteString("Guidelines:\n")
	b.WriteString("- A combat encounter is any scene involving initiative, attacks, spells used offensively, or hostile creatures engaging the party.\n")
	b.WriteString("- Give each encounter a descriptive name (e.g. \"Ambush at the Bridge\", \"Dragon's Lair Battle\").\n")
	b.WriteString("- Set start_time and end_time to the transcript timestamps (in seconds) where combat begins and ends.\n")
	b.WriteString("- Write a brief summary of the encounter outcome.\n")
	b.WriteString("- Extract individual combat actions: attacks, spells, abilities, heals, damage taken, saving throws, and skill checks used in combat.\n")
	b.WriteString("- For each action, identify the actor (who did it), action_type, target (if any), detail (what happened), and damage (if applicable).\n")
	b.WriteString("- action_type must be one of: attack, spell, ability, heal, damage_taken, save, skill\n")
	b.WriteString("- Set round number if you can determine the combat round; otherwise omit it.\n")
	b.WriteString("- Set timestamp to the transcript timestamp in seconds for the action if identifiable.\n")
	b.WriteString("- Use character/NPC names, not player names.\n")
	b.WriteString("- If there are no combat encounters in this session, return an empty encounters array.\n")

	b.WriteString("\nReturn ONLY valid JSON with exactly these fields:\n")
	b.WriteString("{\n")
	b.WriteString("  \"encounters\": [\n")
	b.WriteString("    {\n")
	b.WriteString("      \"name\": \"Encounter Name\",\n")
	b.WriteString("      \"start_time\": 120.0,\n")
	b.WriteString("      \"end_time\": 360.0,\n")
	b.WriteString("      \"summary\": \"Brief outcome description.\",\n")
	b.WriteString("      \"actions\": [\n")
	b.WriteString("        {\"actor\": \"Character Name\", \"action_type\": \"attack\", \"target\": \"Goblin\", \"detail\": \"Swings greatsword\", \"damage\": 12, \"round\": 1, \"timestamp\": 125.0}\n")
	b.WriteString("      ]\n")
	b.WriteString("    }\n")
	b.WriteString("  ]\n")
	b.WriteString("}\n\n")

	b.WriteString("Session Summary:\n")
	b.WriteString(summary)
	b.WriteString("\n\n---\n\n")

	b.WriteString("Transcript:\n")
	b.WriteString(truncateTranscript(transcript, 100000))

	return b.String()
}

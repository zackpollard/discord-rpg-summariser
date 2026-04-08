package summarise

import (
	"context"
	"fmt"
	"strings"
)

// ExtractedCreature represents a creature/monster identified from combat encounters.
type ExtractedCreature struct {
	Name            string `json:"name"`
	CreatureType    string `json:"creature_type"` // beast, undead, fiend, dragon, aberration, etc.
	Description     string `json:"description"`
	ChallengeRating string `json:"challenge_rating"` // "1/4", "3", "20"
	ArmorClass      *int   `json:"armor_class"`
	HitPoints       string `json:"hit_points"` // "52 (8d8+16)"
	Abilities       string `json:"abilities"`  // notable abilities
	Loot            string `json:"loot"`       // drops if mentioned
	Status          string `json:"status"`     // alive, dead, unknown
}

// CreatureExtractionResult holds all creatures extracted from combat encounters.
type CreatureExtractionResult struct {
	Creatures []ExtractedCreature `json:"creatures"`
}

// CreatureExtractor produces creature data from combat encounters.
type CreatureExtractor interface {
	ExtractCreatures(ctx context.Context, transcript, summary string, encounters []CombatExtractedEncounter, playerCharacters []string) (*CreatureExtractionResult, error)
}

// BuildCreatureExtractionPrompt constructs the LLM prompt for identifying and
// extracting creature/monster data from combat encounters.
func BuildCreatureExtractionPrompt(transcript, summary string, encounters []CombatExtractedEncounter, playerCharacters []string) string {
	var b strings.Builder

	b.WriteString("You are an expert D&D 5th Edition bestiary analyst.\n\n")

	if len(playerCharacters) > 0 {
		b.WriteString("The following are PLAYER CHARACTERS — do NOT include them as creatures:\n")
		for _, name := range playerCharacters {
			fmt.Fprintf(&b, "- %s\n", name)
		}
		b.WriteByte('\n')
	}

	b.WriteString("Below is a session transcript, its summary, and extracted combat encounter data.\n")
	b.WriteString("Identify all creatures, monsters, and hostile NPCs that participated in combat.\n\n")

	b.WriteString("Guidelines:\n")
	b.WriteString("- Extract every non-PC combatant from the encounters.\n")
	b.WriteString("- Use the creature's in-game name (e.g. \"Goblin Scout\", \"Red Dragon\", \"Bandit Captain\").\n")
	b.WriteString("- creature_type must be one of: aberration, beast, celestial, construct, dragon, elemental, fey, fiend, giant, humanoid, monstrosity, ooze, plant, undead\n")
	b.WriteString("- Estimate challenge_rating based on the combat difficulty described. Use standard D&D CR values (\"1/8\", \"1/4\", \"1/2\", \"1\", \"2\", etc.).\n")
	b.WriteString("- Estimate armor_class and hit_points based on how hard the creature was to hit and how much damage it took.\n")
	b.WriteString("- List notable abilities used during combat (e.g. \"Multiattack\", \"Fire Breath\", \"Pack Tactics\").\n")
	b.WriteString("- Include any loot or drops mentioned after defeating the creature.\n")
	b.WriteString("- Set status to \"dead\" if the creature was killed, \"alive\" if it escaped or survived, \"unknown\" otherwise.\n")
	b.WriteString("- Group identical creatures: if 3 goblins appear, create one entry for \"Goblin\" not three.\n")
	b.WriteString("- If there are no creatures in combat, return an empty creatures array.\n")

	b.WriteString("\nReturn ONLY valid JSON with exactly these fields:\n")
	b.WriteString("{\n")
	b.WriteString("  \"creatures\": [\n")
	b.WriteString("    {\n")
	b.WriteString("      \"name\": \"Goblin Scout\",\n")
	b.WriteString("      \"creature_type\": \"humanoid\",\n")
	b.WriteString("      \"description\": \"A sneaky goblin scout armed with a shortbow.\",\n")
	b.WriteString("      \"challenge_rating\": \"1/4\",\n")
	b.WriteString("      \"armor_class\": 13,\n")
	b.WriteString("      \"hit_points\": \"7 (2d6)\",\n")
	b.WriteString("      \"abilities\": \"Nimble Escape\",\n")
	b.WriteString("      \"loot\": \"3 gold pieces, shortbow\",\n")
	b.WriteString("      \"status\": \"dead\"\n")
	b.WriteString("    }\n")
	b.WriteString("  ]\n")
	b.WriteString("}\n\n")

	// Include combat encounter data as context.
	if len(encounters) > 0 {
		b.WriteString("Combat Encounters:\n")
		for _, enc := range encounters {
			fmt.Fprintf(&b, "- %s (%.0fs - %.0fs): %s\n", enc.Name, enc.StartTime, enc.EndTime, enc.Summary)
			for _, a := range enc.Actions {
				fmt.Fprintf(&b, "  - %s (%s) → %s: %s", a.Actor, a.ActionType, a.Target, a.Detail)
				if a.Damage != nil {
					fmt.Fprintf(&b, " [%d damage]", *a.Damage)
				}
				b.WriteByte('\n')
			}
		}
		b.WriteByte('\n')
	}

	b.WriteString("Session Summary:\n")
	b.WriteString(summary)
	b.WriteString("\n\n---\n\n")

	b.WriteString("Transcript:\n")
	b.WriteString(truncateTranscript(transcript, 100000))

	return b.String()
}

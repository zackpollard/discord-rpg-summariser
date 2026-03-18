package summarise

import "strings"

// BuildPrompt constructs the full LLM prompt for summarising a D&D session
// transcript. If previousSummary is non-empty it is included as context.
func BuildPrompt(transcript string, previousSummary string) string {
	var b strings.Builder

	b.WriteString("You are an expert summariser for tabletop RPG sessions (Dungeons & Dragons 5th Edition).\n\n")

	if previousSummary != "" {
		b.WriteString("Previously:\n")
		b.WriteString(previousSummary)
		b.WriteString("\n\n---\n\n")
	}

	b.WriteString("Below is the transcript of the latest session. ")
	b.WriteString("Analyse it carefully and produce a structured JSON summary.\n\n")

	b.WriteString("Guidelines:\n")
	b.WriteString("- Use character names, not player names.\n")
	b.WriteString("- Capture all significant plot developments and story beats.\n")
	b.WriteString("- Note any combat encounters, including enemies faced and outcomes.\n")
	b.WriteString("- Identify new lore, world-building details, and revelations.\n")
	b.WriteString("- Record notable NPC interactions and dialogue.\n\n")

	b.WriteString("Return ONLY valid JSON with exactly these fields:\n")
	b.WriteString("{\n")
	b.WriteString("  \"summary\": \"A 2-4 paragraph narrative summary of the session.\",\n")
	b.WriteString("  \"key_events\": [\"Event one\", \"Event two\"],\n")
	b.WriteString("  \"npcs\": [\"Notable NPCs mentioned or encountered\"],\n")
	b.WriteString("  \"places\": [\"Notable places mentioned or visited\"]\n")
	b.WriteString("}\n\n")

	b.WriteString("Transcript:\n")
	b.WriteString(transcript)

	return b.String()
}

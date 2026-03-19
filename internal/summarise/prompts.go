package summarise

import "strings"

// BuildPrompt constructs the full LLM prompt for summarising a D&D session
// transcript. If previousSummary is non-empty it is included as context.
// dmName identifies the Dungeon Master in the transcript (may be empty).
func BuildPrompt(transcript string, previousSummary string, dmName string) string {
	var b strings.Builder

	b.WriteString("You are an expert summariser for tabletop RPG sessions (Dungeons & Dragons 5th Edition).\n\n")

	if dmName != "" {
		b.WriteString("The Dungeon Master for this session is: " + dmName + "\n")
		b.WriteString("When " + dmName + " speaks, they are narrating the world, voicing NPCs, and describing events — not acting as a player character.\n")
		b.WriteString("Attribute NPC dialogue and world descriptions to the NPCs/narrator, not to " + dmName + " personally.\n\n")
	}

	if previousSummary != "" {
		b.WriteString("Previously:\n")
		b.WriteString(previousSummary)
		b.WriteString("\n\n---\n\n")
	}

	b.WriteString("Below is the transcript of the latest session. ")
	b.WriteString("Some entries are marked \"[Name via Telegram]\" — these are text messages the DM sent in a group chat during the session, ")
	b.WriteString("typically containing lore, NPC dialogue, item descriptions, or other detailed information read aloud to the players. ")
	b.WriteString("Treat these Telegram messages as authoritative narration — they are more accurate than the voice transcription of the same content being read aloud.\n\n")
	b.WriteString("Analyse the transcript carefully and produce a structured JSON summary.\n\n")

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

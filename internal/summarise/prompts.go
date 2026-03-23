package summarise

import "strings"

// BuildPrompt constructs the full LLM prompt for summarising a D&D session
// transcript. If previousSummary is non-empty it is included as context.
// dmName is unused (kept for API compatibility) — the DM is always labelled
// "DM" in the transcript.
func BuildPrompt(transcript string, previousSummary string, dmName string) string {
	var b strings.Builder

	b.WriteString("You are an expert summariser for tabletop RPG sessions (Dungeons & Dragons 5th Edition).\n\n")

	b.WriteString("Lines attributed to \"DM\" are the Dungeon Master speaking. The DM performs multiple roles:\n")
	b.WriteString("- Narrating the world, describing scenes, environments, and events\n")
	b.WriteString("- Voicing NPCs in character (e.g., when the DM says \"I am Lord Strahd\", attribute this to Strahd, not the DM)\n")
	b.WriteString("- Adjudicating rules, describing combat outcomes, and asking players for rolls\n")
	b.WriteString("When summarising, attribute NPC dialogue to the NPC who is speaking, not to \"the DM\".\n")
	b.WriteString("Use context clues to identify which NPC the DM is voicing (tone shifts, name references, direct speech patterns).\n\n")

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

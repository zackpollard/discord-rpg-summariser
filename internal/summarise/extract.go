package summarise

import (
	"context"
	"fmt"
	"strings"
)

// ExtractedEntity represents a single entity extracted from a session transcript.
type ExtractedEntity struct {
	Name         string `json:"name"`
	Type         string `json:"type"` // npc, place, organisation, item, event
	Description  string `json:"description"`
	Notes        string `json:"notes"`          // what happened THIS session
	Status       string `json:"status"`         // alive, dead, unknown
	CauseOfDeath string `json:"cause_of_death"` // if dead, how they died
	ParentPlace  string `json:"parent_place"`   // for places: the containing place name
}

// ExtractedRelationship represents a relationship between two extracted entities.
type ExtractedRelationship struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	Relationship string `json:"relationship"` // allied_with, enemy_of, located_in, member_of, owns, related_to
	Description  string `json:"description"`
}

// ExtractionResult holds all entities and relationships extracted from a session.
type ExtractionResult struct {
	Entities      []ExtractedEntity       `json:"entities"`
	Relationships []ExtractedRelationship `json:"relationships"`
}

// EntityExtractor produces structured entity and relationship data from a session transcript.
type EntityExtractor interface {
	ExtractEntities(ctx context.Context, transcript, summary string, existingEntities []string, dmName string, playerCharacters []string) (*ExtractionResult, error)
}

// BuildExtractionPrompt constructs the LLM prompt for extracting entities and
// relationships from a session transcript and its summary.
func BuildExtractionPrompt(transcript, summary string, existingEntities []string, dmName string, playerCharacters []string) string {
	var b strings.Builder

	b.WriteString("You are an expert lore-keeper for tabletop RPG campaigns (Dungeons & Dragons 5th Edition).\n\n")

	if dmName != "" {
		b.WriteString("The Dungeon Master is: " + dmName + "\n")
		b.WriteString("Everything " + dmName + " says is narration, NPC dialogue, or world description — not a player character.\n")
		b.WriteString("Extract NPCs and lore from the DM's narration. Do NOT create an entity for the DM themselves.\n\n")
	}

	b.WriteString("Below is a session transcript and its summary. ")
	b.WriteString("Analyse them carefully and extract all notable entities and their relationships.\n\n")

	if len(playerCharacters) > 0 {
		b.WriteString("The following are PLAYER CHARACTERS — Do NOT extract player characters as entities (they already exist as type 'pc'). ")
		b.WriteString("However, DO include relationships where a player character is the source or target. ")
		b.WriteString("Use the exact character names listed below for source/target fields:\n")
		for _, name := range playerCharacters {
			fmt.Fprintf(&b, "- %s\n", name)
		}
		b.WriteByte('\n')
	}

	b.WriteString("Guidelines:\n")
	b.WriteString("- Extract NPCs, places, organisations, items, and events mentioned in the session.\n")
	b.WriteString("- Do NOT extract player characters as entities — they already exist.\n")
	b.WriteString("- Use character names, not player names.\n")
	b.WriteString("- For each entity, write a concise description (what it IS) and notes (what happened THIS session).\n")
	b.WriteString("- For each NPC, include their current status: 'alive', 'dead', or 'unknown'. If they died this session or previously, include cause_of_death.\n")
	b.WriteString("- For non-NPC entities (places, items, etc.), set status to 'unknown' unless destruction/loss is relevant.\n")
	b.WriteString("- For place entities, if they are located within another place, set `parent_place` to the name of the containing place.\n")
	b.WriteString("- Identify relationships between entities: allied_with, enemy_of, located_in, member_of, owns, related_to.\n")
	b.WriteString("- Source and target in relationships must exactly match entity names or player character names.\n")

	if len(existingEntities) > 0 {
		b.WriteString("\nThe following entities already exist in the knowledge base. ")
		b.WriteString("Use these exact names when referring to them (do not create duplicates):\n")
		for _, name := range existingEntities {
			fmt.Fprintf(&b, "- %s\n", name)
		}
	}

	b.WriteString("\nEntity types: npc, place, organisation, item, event\n")
	b.WriteString("Relationship types: allied_with, enemy_of, located_in, member_of, owns, related_to\n\n")

	b.WriteString("Return ONLY valid JSON with exactly these fields:\n")
	b.WriteString("{\n")
	b.WriteString("  \"entities\": [\n")
	b.WriteString("    {\"name\": \"Entity Name\", \"type\": \"npc\", \"description\": \"What it is.\", \"notes\": \"What happened this session.\", \"status\": \"alive\", \"cause_of_death\": \"\", \"parent_place\": \"\"}\n")
	b.WriteString("  ],\n")
	b.WriteString("  \"relationships\": [\n")
	b.WriteString("    {\"source\": \"Entity A\", \"target\": \"Entity B\", \"relationship\": \"allied_with\", \"description\": \"Brief description.\"}\n")
	b.WriteString("  ]\n")
	b.WriteString("}\n\n")

	b.WriteString("Session Summary:\n")
	b.WriteString(summary)
	b.WriteString("\n\n---\n\n")

	b.WriteString("Transcript:\n")
	b.WriteString(transcript)

	return b.String()
}

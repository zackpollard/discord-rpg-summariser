package summarise

import (
	"context"
	"fmt"
	"strings"
)

// ExtractedEntity represents a single entity extracted from a session transcript.
type ExtractedEntity struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // npc, place, organisation, item, event
	Description string `json:"description"`
	Notes       string `json:"notes"` // what happened THIS session
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
	ExtractEntities(ctx context.Context, transcript, summary string, existingEntities []string, dmName string) (*ExtractionResult, error)
}

// BuildExtractionPrompt constructs the LLM prompt for extracting entities and
// relationships from a session transcript and its summary.
func BuildExtractionPrompt(transcript, summary string, existingEntities []string, dmName string) string {
	var b strings.Builder

	b.WriteString("You are an expert lore-keeper for tabletop RPG campaigns (Dungeons & Dragons 5th Edition).\n\n")

	if dmName != "" {
		b.WriteString("The Dungeon Master is: " + dmName + "\n")
		b.WriteString("Everything " + dmName + " says is narration, NPC dialogue, or world description — not a player character.\n")
		b.WriteString("Extract NPCs and lore from the DM's narration. Do NOT create an entity for the DM themselves.\n\n")
	}

	b.WriteString("Below is a session transcript and its summary. ")
	b.WriteString("Analyse them carefully and extract all notable entities and their relationships.\n\n")

	b.WriteString("Guidelines:\n")
	b.WriteString("- Extract NPCs, places, organisations, items, and events mentioned in the session.\n")
	b.WriteString("- Use character names, not player names.\n")
	b.WriteString("- For each entity, write a concise description (what it IS) and notes (what happened THIS session).\n")
	b.WriteString("- Identify relationships between entities: allied_with, enemy_of, located_in, member_of, owns, related_to.\n")
	b.WriteString("- Source and target in relationships must exactly match entity names.\n")

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
	b.WriteString("    {\"name\": \"Entity Name\", \"type\": \"npc\", \"description\": \"What it is.\", \"notes\": \"What happened this session.\"}\n")
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

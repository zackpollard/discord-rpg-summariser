package summarise

import (
	"context"
	"fmt"
	"strings"
)

// RecapResult holds the structured output from an LLM recap generation.
type RecapResult struct {
	Recap string `json:"recap"`
}

// RecapGenerator produces a "story so far" narrative from session summaries.
type RecapGenerator interface {
	GenerateRecap(ctx context.Context, sessionSummaries []string, dmName string) (*RecapResult, error)
}

// BuildRecapPrompt constructs the LLM prompt for generating a "story so far"
// narrative recap from chronological session summaries.
func BuildRecapPrompt(sessionSummaries []string, dmName string) string {
	var b strings.Builder

	b.WriteString("You are an expert storyteller and lore-keeper for tabletop RPG campaigns (Dungeons & Dragons 5th Edition).\n\n")

	if dmName != "" {
		b.WriteString("The Dungeon Master is: " + dmName + "\n\n")
	}

	b.WriteString("Below are summaries of each session played so far, in chronological order. ")
	b.WriteString("Synthesise them into a cohesive \"Story So Far\" narrative recap.\n\n")

	b.WriteString("Guidelines:\n")
	b.WriteString("- Write in past tense, third person.\n")
	b.WriteString("- Use character names, not player names.\n")
	b.WriteString("- Weave the sessions into a flowing narrative, not a list of session-by-session bullet points.\n")
	b.WriteString("- Highlight major plot developments, key decisions, and turning points.\n")
	b.WriteString("- Mention important NPCs, locations, and items as they become relevant.\n")
	b.WriteString("- Keep it engaging and readable — this will be shown to players as a refresher.\n")
	b.WriteString("- Aim for a length proportional to the number of sessions (roughly 1-2 paragraphs per session).\n\n")

	b.WriteString("Return ONLY valid JSON with exactly this field:\n")
	b.WriteString("{\n")
	b.WriteString("  \"recap\": \"The full narrative recap text.\"\n")
	b.WriteString("}\n\n")

	b.WriteString("Session Summaries (chronological order):\n\n")
	for i, summary := range sessionSummaries {
		fmt.Fprintf(&b, "--- Session %d ---\n%s\n\n", i+1, summary)
	}

	return b.String()
}

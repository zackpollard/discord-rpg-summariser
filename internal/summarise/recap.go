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
	GenerateRecap(ctx context.Context, sessionSummaries []string, dmName string, style ...string) (*RecapResult, error)
}

// RecapPromptOptions holds optional parameters for BuildRecapPrompt.
type RecapPromptOptions struct {
	LastN int
	Style string // "dramatic", "casual", "in-character", or "" for default
}

// BuildRecapPrompt constructs the LLM prompt for generating a "story so far"
// narrative recap from chronological session summaries.
// If lastN > 0, the prompt is adjusted to indicate a partial recap covering
// only the most recent N sessions.
func BuildRecapPrompt(sessionSummaries []string, dmName string, opts ...RecapPromptOptions) string {
	var o RecapPromptOptions
	if len(opts) > 0 {
		o = opts[0]
	}

	var b strings.Builder

	// Prepend style instructions if a non-default style is specified.
	switch o.Style {
	case "dramatic":
		b.WriteString("IMPORTANT STYLE INSTRUCTION: Write in a dramatic, epic fantasy narrator voice. Use vivid, cinematic language. Build tension, emphasize heroic moments, and make the narrative feel like the opening crawl of a fantasy epic. Use rich metaphors and evocative descriptions.\n\n")
	case "casual":
		b.WriteString("IMPORTANT STYLE INSTRUCTION: Write in a casual, informal tone. Summarise events like you're telling a friend what happened over coffee. Use relaxed language, contractions, and a conversational style. Keep it fun and approachable.\n\n")
	case "in-character":
		b.WriteString("IMPORTANT STYLE INSTRUCTION: Write as if you are an NPC chronicler or bard within the game world. Use first-person perspective of a storyteller NPC. Include flourishes like \"Dear reader\" or \"As I witnessed...\" — make it feel like an in-world historical document or tavern tale.\n\n")
	}

	b.WriteString("You are an expert storyteller and lore-keeper for tabletop RPG campaigns (Dungeons & Dragons 5th Edition).\n\n")

	if dmName != "" {
		b.WriteString("The Dungeon Master is: " + dmName + "\n\n")
	}

	if o.LastN > 0 {
		fmt.Fprintf(&b, "Below are summaries of the most recent %d sessions, in chronological order. ", o.LastN)
		b.WriteString("Synthesise them into a cohesive narrative recap of recent events.\n\n")
	} else {
		b.WriteString("Below are summaries of each session played so far, in chronological order. ")
		b.WriteString("Synthesise them into a cohesive \"Story So Far\" narrative recap.\n\n")
	}

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

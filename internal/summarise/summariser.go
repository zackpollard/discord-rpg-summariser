package summarise

import "context"

// SummaryResult holds the structured output from an LLM summarisation.
type SummaryResult struct {
	Summary   string   `json:"summary"`
	KeyEvents []string `json:"key_events"`
	NPCs      []string `json:"npcs"`
	Places    []string `json:"places"`
}

// Summariser produces a structured summary from a session transcript.
type Summariser interface {
	Summarise(ctx context.Context, transcript string, previousSummary string, dmName string) (*SummaryResult, error)
}

package summarise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ClaudeCLI implements Summariser by shelling out to the `claude` CLI tool.
type ClaudeCLI struct{}

// NewClaudeCLI creates a new ClaudeCLI summariser.
func NewClaudeCLI() *ClaudeCLI {
	return &ClaudeCLI{}
}

// runPrompt executes the claude CLI with the given prompt and unmarshals the
// JSON response into result. This eliminates duplication across all extraction
// methods which follow the identical pattern: build prompt, run CLI, parse JSON.
func (c *ClaudeCLI) runPrompt(ctx context.Context, prompt string, result any) error {
	cmd := exec.CommandContext(ctx, "claude", "--print")
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude CLI failed: %w: %s", err, stderr.String())
	}

	output := StripCodeFences(stdout.Bytes())

	if err := json.Unmarshal(output, result); err != nil {
		return fmt.Errorf("parse claude CLI JSON response: %w\nraw output: %s", err, stdout.String())
	}

	return nil
}

// Summarise runs the claude CLI with the built prompt piped via stdin and
// parses the JSON response into a SummaryResult.
func (c *ClaudeCLI) Summarise(ctx context.Context, transcript string, previousSummary string, dmName string) (*SummaryResult, error) {
	prompt := BuildPrompt(transcript, previousSummary, dmName)
	var result SummaryResult
	if err := c.runPrompt(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtractEntities runs the claude CLI with the extraction prompt and parses
// the JSON response into an ExtractionResult.
func (c *ClaudeCLI) ExtractEntities(ctx context.Context, transcript, summary string, existingEntities []string, dmName string, playerCharacters []string) (*ExtractionResult, error) {
	prompt := BuildExtractionPrompt(transcript, summary, existingEntities, dmName, playerCharacters)
	var result ExtractionResult
	if err := c.runPrompt(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtractQuests runs the claude CLI with the quest extraction prompt and
// parses the JSON response into a QuestExtractionResult.
func (c *ClaudeCLI) ExtractQuests(ctx context.Context, transcript, summary string, existingQuests []string, dmName string) (*QuestExtractionResult, error) {
	prompt := BuildQuestExtractionPrompt(transcript, summary, existingQuests, dmName)
	var result QuestExtractionResult
	if err := c.runPrompt(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GenerateRecap runs the claude CLI with the recap prompt and parses the
// JSON response into a RecapResult.
func (c *ClaudeCLI) GenerateRecap(ctx context.Context, sessionSummaries []string, dmName string) (*RecapResult, error) {
	prompt := BuildRecapPrompt(sessionSummaries, dmName)
	var result RecapResult
	if err := c.runPrompt(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtractCombat runs the claude CLI with the combat extraction prompt and
// parses the JSON response into a CombatExtractionResult.
func (c *ClaudeCLI) ExtractCombat(ctx context.Context, transcript, summary, dmName string, playerCharacters []string) (*CombatExtractionResult, error) {
	prompt := BuildCombatExtractionPrompt(transcript, summary, dmName, playerCharacters)
	var result CombatExtractionResult
	if err := c.runPrompt(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// StripCodeFences removes optional ```json ... ``` wrapping from LLM output.
func StripCodeFences(b []byte) []byte {
	s := strings.TrimSpace(string(b))
	if strings.HasPrefix(s, "```") {
		// Remove opening fence (possibly ```json).
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove closing fence.
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
	}
	return []byte(strings.TrimSpace(s))
}

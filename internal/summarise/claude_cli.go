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

// Summarise runs the claude CLI with the built prompt piped via stdin and
// parses the JSON response into a SummaryResult.
func (c *ClaudeCLI) Summarise(ctx context.Context, transcript string, previousSummary string, dmName string) (*SummaryResult, error) {
	prompt := BuildPrompt(transcript, previousSummary, dmName)

	cmd := exec.CommandContext(ctx, "claude", "--print")
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude CLI failed: %w: %s", err, stderr.String())
	}

	output := stdout.Bytes()

	// The CLI may wrap the JSON in markdown fences; strip them.
	output = StripCodeFences(output)

	var result SummaryResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse claude CLI JSON response: %w\nraw output: %s", err, stdout.String())
	}

	return &result, nil
}

// ExtractEntities runs the claude CLI with the extraction prompt and parses
// the JSON response into an ExtractionResult.
func (c *ClaudeCLI) ExtractEntities(ctx context.Context, transcript, summary string, existingEntities []string, dmName string) (*ExtractionResult, error) {
	prompt := BuildExtractionPrompt(transcript, summary, existingEntities, dmName)

	cmd := exec.CommandContext(ctx, "claude", "--print")
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude CLI failed: %w: %s", err, stderr.String())
	}

	output := stdout.Bytes()
	output = StripCodeFences(output)

	var result ExtractionResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse claude CLI extraction JSON: %w\nraw output: %s", err, stdout.String())
	}

	return &result, nil
}

// ExtractQuests runs the claude CLI with the quest extraction prompt and
// parses the JSON response into a QuestExtractionResult.
func (c *ClaudeCLI) ExtractQuests(ctx context.Context, transcript, summary string, existingQuests []string, dmName string) (*QuestExtractionResult, error) {
	prompt := BuildQuestExtractionPrompt(transcript, summary, existingQuests, dmName)

	cmd := exec.CommandContext(ctx, "claude", "--print")
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude CLI failed: %w: %s", err, stderr.String())
	}

	output := stdout.Bytes()
	output = StripCodeFences(output)

	var result QuestExtractionResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse claude CLI quest extraction JSON: %w\nraw output: %s", err, stdout.String())
	}

	return &result, nil
}

// GenerateRecap runs the claude CLI with the recap prompt and parses the
// JSON response into a RecapResult.
func (c *ClaudeCLI) GenerateRecap(ctx context.Context, sessionSummaries []string, dmName string) (*RecapResult, error) {
	prompt := BuildRecapPrompt(sessionSummaries, dmName)

	cmd := exec.CommandContext(ctx, "claude", "--print")
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude CLI failed: %w: %s", err, stderr.String())
	}

	output := stdout.Bytes()
	output = StripCodeFences(output)

	var result RecapResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse claude CLI recap JSON: %w\nraw output: %s", err, stdout.String())
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

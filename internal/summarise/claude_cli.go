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
func (c *ClaudeCLI) Summarise(ctx context.Context, transcript string, previousSummary string) (*SummaryResult, error) {
	prompt := BuildPrompt(transcript, previousSummary)

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
	output = stripCodeFences(output)

	var result SummaryResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse claude CLI JSON response: %w\nraw output: %s", err, stdout.String())
	}

	return &result, nil
}

// stripCodeFences removes optional ```json ... ``` wrapping from LLM output.
func stripCodeFences(b []byte) []byte {
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

package summarise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// LLMLogEntry contains the data captured for a single LLM call.
type LLMLogEntry struct {
	Operation  string
	Prompt     string
	Response   string
	Error      string
	DurationMS int
}

// LogFunc is called after each LLM invocation with the captured data.
// The context carries the session ID set by the caller.
type LogFunc func(ctx context.Context, entry LLMLogEntry)

// ClaudeCLI implements Summariser by shelling out to the `claude` CLI tool.
type ClaudeCLI struct {
	OnLog LogFunc
}

// NewClaudeCLI creates a new ClaudeCLI summariser.
func NewClaudeCLI() *ClaudeCLI {
	return &ClaudeCLI{}
}

// runPrompt executes the claude CLI with the given prompt and unmarshals the
// JSON response into result. This eliminates duplication across all extraction
// methods which follow the identical pattern: build prompt, run CLI, parse JSON.
func (c *ClaudeCLI) runPrompt(ctx context.Context, operation, prompt string, result any) error {
	start := time.Now()

	cmd := exec.CommandContext(ctx, "claude", "--print", "--model", "claude-opus-4-6", "--effort", "max")
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	durationMS := int(time.Since(start).Milliseconds())
	response := stdout.String()

	if runErr != nil {
		errMsg := fmt.Sprintf("claude CLI failed: %v\nstderr: %s\nstdout: %s", runErr, stderr.String(), response)
		c.log(ctx, LLMLogEntry{
			Operation:  operation,
			Prompt:     prompt,
			Response:   response,
			Error:      errMsg,
			DurationMS: durationMS,
		})
		return fmt.Errorf("%s", errMsg)
	}

	output := StripCodeFences(stdout.Bytes())

	if err := json.Unmarshal(output, result); err != nil {
		errMsg := fmt.Sprintf("parse claude CLI JSON response: %v\nraw output: %s", err, response)
		c.log(ctx, LLMLogEntry{
			Operation:  operation,
			Prompt:     prompt,
			Response:   response,
			Error:      errMsg,
			DurationMS: durationMS,
		})
		return fmt.Errorf("%s", errMsg)
	}

	c.log(ctx, LLMLogEntry{
		Operation:  operation,
		Prompt:     prompt,
		Response:   response,
		DurationMS: durationMS,
	})

	return nil
}

func (c *ClaudeCLI) log(ctx context.Context, entry LLMLogEntry) {
	if c.OnLog != nil {
		c.OnLog(ctx, entry)
	}
}

// Summarise runs the claude CLI with the built prompt piped via stdin and
// parses the JSON response into a SummaryResult.
func (c *ClaudeCLI) Summarise(ctx context.Context, transcript string, previousSummary string, dmName string) (*SummaryResult, error) {
	prompt := BuildPrompt(transcript, previousSummary, dmName)
	var result SummaryResult
	if err := c.runPrompt(ctx, "summarise", prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtractEntities runs the claude CLI with the extraction prompt and parses
// the JSON response into an ExtractionResult.
func (c *ClaudeCLI) ExtractEntities(ctx context.Context, transcript, summary string, existingEntities []string, dmName string, playerCharacters []string) (*ExtractionResult, error) {
	prompt := BuildExtractionPrompt(transcript, summary, existingEntities, dmName, playerCharacters)
	var result ExtractionResult
	if err := c.runPrompt(ctx, "extract_entities", prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtractQuests runs the claude CLI with the quest extraction prompt and
// parses the JSON response into a QuestExtractionResult.
func (c *ClaudeCLI) ExtractQuests(ctx context.Context, transcript, summary string, existingQuests []string, dmName string) (*QuestExtractionResult, error) {
	prompt := BuildQuestExtractionPrompt(transcript, summary, existingQuests, dmName)
	var result QuestExtractionResult
	if err := c.runPrompt(ctx, "extract_quests", prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GenerateRecap runs the claude CLI with the recap prompt and parses the
// JSON response into a RecapResult.
func (c *ClaudeCLI) GenerateRecap(ctx context.Context, sessionSummaries []string, dmName string) (*RecapResult, error) {
	prompt := BuildRecapPrompt(sessionSummaries, dmName)
	var result RecapResult
	if err := c.runPrompt(ctx, "generate_recap", prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtractCombat runs the claude CLI with the combat extraction prompt and
// parses the JSON response into a CombatExtractionResult.
func (c *ClaudeCLI) ExtractCombat(ctx context.Context, transcript, summary, dmName string, playerCharacters []string) (*CombatExtractionResult, error) {
	prompt := BuildCombatExtractionPrompt(transcript, summary, dmName, playerCharacters)
	var result CombatExtractionResult
	if err := c.runPrompt(ctx, "extract_combat", prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtractTitleAndQuotes runs the claude CLI to generate a session title and
// extract memorable quotes from the transcript.
func (c *ClaudeCLI) ExtractTitleAndQuotes(ctx context.Context, transcript, summary, dmName string) (*TitleAndQuotesResult, error) {
	prompt := BuildTitleAndQuotesPrompt(transcript, summary, dmName)
	var result TitleAndQuotesResult
	if err := c.runPrompt(ctx, "extract_title_quotes", prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AnnotateTranscript runs the claude CLI with the annotation prompt.
func (c *ClaudeCLI) AnnotateTranscript(ctx context.Context, segments []AnnotationInput, vocab AnnotationVocabulary, dmName string) (*AnnotationResult, error) {
	prompt := BuildAnnotationPrompt(segments, vocab, dmName)
	var result AnnotationResult
	if err := c.runPrompt(ctx, "annotate_transcript", prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// StripCodeFences extracts JSON from LLM output that may contain free text,
// code fences, or both. It handles:
//   - Pure JSON: {"key": "value"}
//   - Code fences: ```json\n{...}\n```
//   - Text before code fences: "Here is the result:\n```json\n{...}\n```"
//   - Text before raw JSON: "Looking at...\n\n{"key": "value"}"
func StripCodeFences(b []byte) []byte {
	s := strings.TrimSpace(string(b))

	// If there's a code fence anywhere, extract its content.
	if fenceStart := strings.Index(s, "```"); fenceStart >= 0 {
		inner := s[fenceStart+3:]
		// Skip the language tag line (e.g. "json\n").
		if nl := strings.Index(inner, "\n"); nl >= 0 {
			inner = inner[nl+1:]
		}
		// Find closing fence.
		if fenceEnd := strings.LastIndex(inner, "```"); fenceEnd >= 0 {
			inner = inner[:fenceEnd]
		}
		return []byte(strings.TrimSpace(inner))
	}

	// No code fence — try to find raw JSON by locating the first { or [.
	for i, c := range s {
		if c == '{' || c == '[' {
			// Find the matching closing bracket.
			candidate := s[i:]
			return []byte(strings.TrimSpace(candidate))
		}
	}

	return []byte(s)
}

package summarise

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
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

// LLMStreamFunc is called with real-time log lines from the LLM process.
type LLMStreamFunc func(operation, message string)

// ClaudeCLI implements Summariser by shelling out to the `claude` CLI tool.
type ClaudeCLI struct {
	OnLog    LogFunc
	OnStream LLMStreamFunc // called with real-time stderr lines during generation
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

	log.Printf("llm: starting %s (prompt: %d chars)", operation, len(prompt))

	cmd := exec.CommandContext(ctx, "claude", "--print", "--model", "claude-opus-4-6", "--effort", "max",
		"--output-format", "stream-json", "--verbose", "--include-partial-messages")
	cmd.Stdin = strings.NewReader(prompt)

	stdoutPipe, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		return fmt.Errorf("stdout pipe: %w", pipeErr)
	}
	cmd.Stderr = nil // stream-json outputs everything to stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	// Parse streaming JSON events from stdout.
	var response string
	var textAccum strings.Builder
	var inputTokens, outputTokens int
	var costUSD float64
	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for large responses
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event struct {
			Type  string `json:"type"`
			Event struct {
				Type    string `json:"type"`
				Delta   struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
				Message struct {
					Usage struct {
						InputTokens  int `json:"input_tokens"`
						OutputTokens int `json:"output_tokens"`
					} `json:"usage"`
				} `json:"message"`
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"event"`
			Result       string  `json:"result"`
			TotalCostUSD float64 `json:"total_cost_usd"`
			Usage        struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "stream_event":
			switch event.Event.Type {
			case "content_block_delta":
				if event.Event.Delta.Type == "text_delta" {
					chunk := event.Event.Delta.Text
					textAccum.WriteString(chunk)

					if c.OnStream != nil {
						full := textAccum.String()
						preview := full
						if len(preview) > 200 {
							preview = "..." + preview[len(preview)-200:]
						}
						tokens := fmt.Sprintf(" [%d tokens]", outputTokens)
						c.OnStream(operation, preview+tokens)
					}
				}
			case "message_start":
				inputTokens = event.Event.Message.Usage.InputTokens
				if c.OnStream != nil {
					c.OnStream(operation, fmt.Sprintf("Processing %d input tokens...", inputTokens))
				}
			case "message_delta":
				outputTokens = event.Event.Usage.OutputTokens
			}
		case "result":
			response = event.Result
			costUSD = event.TotalCostUSD
			if event.Usage.InputTokens > 0 {
				inputTokens = event.Usage.InputTokens
			}
			if event.Usage.OutputTokens > 0 {
				outputTokens = event.Usage.OutputTokens
			}
		}
	}

	// If no result event, use accumulated text.
	if response == "" {
		response = textAccum.String()
	}

	log.Printf("llm: %s tokens: in=%d out=%d cost=$%.4f",
		operation, inputTokens, outputTokens, costUSD)

	runErr := cmd.Wait()
	durationMS := int(time.Since(start).Milliseconds())

	log.Printf("llm: %s completed in %.1fs (response: %d chars, err: %v)",
		operation, float64(durationMS)/1000, len(response), runErr)

	if runErr != nil {
		errMsg := fmt.Sprintf("claude CLI failed: %v\nresponse: %s", runErr, response)
		c.log(ctx, LLMLogEntry{
			Operation:  operation,
			Prompt:     prompt,
			Response:   response,
			Error:      errMsg,
			DurationMS: durationMS,
		})
		return fmt.Errorf("%s", errMsg)
	}

	output := StripCodeFences([]byte(response))

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
func (c *ClaudeCLI) GenerateRecap(ctx context.Context, sessionSummaries []string, dmName string, style ...string) (*RecapResult, error) {
	opts := RecapPromptOptions{}
	if len(style) > 0 {
		opts.Style = style[0]
	}
	prompt := BuildRecapPrompt(sessionSummaries, dmName, opts)
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

// GeneratePreviouslyOn runs the claude CLI with the "Previously on..." prompt.
func (c *ClaudeCLI) GeneratePreviouslyOn(ctx context.Context, lastSessionSummary, campaignRecap string) (*PreviouslyOnResult, error) {
	prompt := BuildPreviouslyOnPrompt(lastSessionSummary, campaignRecap)
	var result PreviouslyOnResult
	if err := c.runPrompt(ctx, "generate_previously_on", prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GenerateCharacterSummary runs the claude CLI with the character summary prompt.
func (c *ClaudeCLI) GenerateCharacterSummary(ctx context.Context, characterName string, sessionSummaries []string, relationships []string) (*CharacterSummaryResult, error) {
	prompt := BuildCharacterSummaryPrompt(characterName, sessionSummaries, relationships)
	var result CharacterSummaryResult
	if err := c.runPrompt(ctx, "generate_character_summary", prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AnalyzeCombat runs the claude CLI with the combat analysis prompt.
func (c *ClaudeCLI) AnalyzeCombat(ctx context.Context, encounterSummary string, actions []string, playerCharacters []string) (*CombatAnalysisResult, error) {
	prompt := BuildCombatAnalysisPrompt(encounterSummary, actions, playerCharacters)
	var result CombatAnalysisResult
	if err := c.runPrompt(ctx, "analyze_combat", prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SuggestClipNames runs the claude CLI with the clip name suggestion prompt.
func (c *ClaudeCLI) SuggestClipNames(ctx context.Context, transcriptExcerpt string) (*ClipNameResult, error) {
	prompt := BuildClipNamePrompt(transcriptExcerpt)
	var result ClipNameResult
	if err := c.runPrompt(ctx, "suggest_clip_names", prompt, &result); err != nil {
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

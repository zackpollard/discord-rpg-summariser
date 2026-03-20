package summarise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Ollama implements Summariser using a local Ollama instance.
type Ollama struct {
	url   string // base URL, e.g. "http://localhost:11434"
	model string // model name, e.g. "llama3"
}

// NewOllama creates a new Ollama summariser.
func NewOllama(url, model string) *Ollama {
	return &Ollama{
		url:   url,
		model: model,
	}
}

// ollamaRequest is the body sent to /api/generate.
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format"`
}

// ollamaResponse is the shape returned by /api/generate (non-streaming).
type ollamaResponse struct {
	Response string `json:"response"`
}

// runPrompt sends the given prompt to Ollama and unmarshals the JSON response
// into result. This eliminates duplication across all extraction methods which
// follow the identical pattern: build prompt, call Ollama, parse JSON.
func (o *Ollama) runPrompt(ctx context.Context, prompt string, result any) error {
	reqBody := ollamaRequest{
		Model:  o.model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url+"/api/generate", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read ollama response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return fmt.Errorf("parse ollama response envelope: %w", err)
	}

	if err := json.Unmarshal([]byte(ollamaResp.Response), result); err != nil {
		return fmt.Errorf("parse ollama JSON: %w\nraw response: %s", err, ollamaResp.Response)
	}

	return nil
}

// Summarise sends the prompt to Ollama and parses the JSON response.
func (o *Ollama) Summarise(ctx context.Context, transcript string, previousSummary string, dmName string) (*SummaryResult, error) {
	prompt := BuildPrompt(transcript, previousSummary, dmName)
	var result SummaryResult
	if err := o.runPrompt(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtractEntities sends the extraction prompt to Ollama and parses the JSON response.
func (o *Ollama) ExtractEntities(ctx context.Context, transcript, summary string, existingEntities []string, dmName string, playerCharacters []string) (*ExtractionResult, error) {
	prompt := BuildExtractionPrompt(transcript, summary, existingEntities, dmName, playerCharacters)
	var result ExtractionResult
	if err := o.runPrompt(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtractQuests sends the quest extraction prompt to Ollama and parses the JSON response.
func (o *Ollama) ExtractQuests(ctx context.Context, transcript, summary string, existingQuests []string, dmName string) (*QuestExtractionResult, error) {
	prompt := BuildQuestExtractionPrompt(transcript, summary, existingQuests, dmName)
	var result QuestExtractionResult
	if err := o.runPrompt(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtractCombat sends the combat extraction prompt to Ollama and parses the JSON response.
func (o *Ollama) ExtractCombat(ctx context.Context, transcript, summary, dmName string, playerCharacters []string) (*CombatExtractionResult, error) {
	prompt := BuildCombatExtractionPrompt(transcript, summary, dmName, playerCharacters)
	var result CombatExtractionResult
	if err := o.runPrompt(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GenerateRecap sends the recap prompt to Ollama and parses the JSON response.
func (o *Ollama) GenerateRecap(ctx context.Context, sessionSummaries []string, dmName string) (*RecapResult, error) {
	prompt := BuildRecapPrompt(sessionSummaries, dmName)
	var result RecapResult
	if err := o.runPrompt(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

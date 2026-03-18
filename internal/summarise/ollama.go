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

// Summarise sends the prompt to Ollama and parses the JSON response.
func (o *Ollama) Summarise(ctx context.Context, transcript string, previousSummary string) (*SummaryResult, error) {
	prompt := BuildPrompt(transcript, previousSummary)

	reqBody := ollamaRequest{
		Model:  o.model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url+"/api/generate", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ollama response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("parse ollama response envelope: %w", err)
	}

	var result SummaryResult
	if err := json.Unmarshal([]byte(ollamaResp.Response), &result); err != nil {
		return nil, fmt.Errorf("parse ollama summary JSON: %w\nraw response: %s", err, ollamaResp.Response)
	}

	return &result, nil
}

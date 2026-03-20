package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OllamaEmbedder implements Embedder using a local Ollama instance.
type OllamaEmbedder struct {
	url   string // base URL, e.g. "http://localhost:11434"
	model string // model name, e.g. "nomic-embed-text"
}

// NewOllamaEmbedder creates a new Ollama-backed embedder.
func NewOllamaEmbedder(url, model string) *OllamaEmbedder {
	return &OllamaEmbedder{
		url:   url,
		model: model,
	}
}

// ollamaEmbedRequest is the body sent to POST /api/embed.
type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"` // string for single, []string for batch
}

// ollamaEmbedResponse is the response from POST /api/embed.
type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// Embed returns the embedding vector for a single text string.
func (o *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := o.doEmbed(ctx, text)
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("ollama embed: no embeddings returned")
	}
	return vecs[0], nil
}

// EmbedBatch returns embedding vectors for multiple text strings.
func (o *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if len(texts) == 1 {
		vec, err := o.Embed(ctx, texts[0])
		if err != nil {
			return nil, err
		}
		return [][]float32{vec}, nil
	}
	return o.doEmbed(ctx, texts)
}

// doEmbed sends an embed request to Ollama. input can be a string or []string.
func (o *OllamaEmbedder) doEmbed(ctx context.Context, input any) ([][]float32, error) {
	reqBody := ollamaEmbedRequest{
		Model: o.model,
		Input: input,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal ollama embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url+"/api/embed", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create ollama embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ollama embed response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var embedResp ollamaEmbedResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, fmt.Errorf("parse ollama embed response: %w", err)
	}

	return embedResp.Embeddings, nil
}

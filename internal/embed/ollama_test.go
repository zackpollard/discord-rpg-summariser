package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaEmbedder_Embed(t *testing.T) {
	// Mock Ollama server returning a 768-dim embedding.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req ollamaEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "nomic-embed-text" {
			t.Errorf("unexpected model: %s", req.Model)
		}

		// Return a mock embedding.
		vec := make([]float32, 768)
		for i := range vec {
			vec[i] = float32(i) * 0.001
		}

		resp := ollamaEmbedResponse{
			Embeddings: [][]float32{vec},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "nomic-embed-text")
	vec, err := embedder.Embed(context.Background(), "Hello, world!")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(vec) != 768 {
		t.Errorf("expected 768-dim vector, got %d", len(vec))
	}
}

func TestOllamaEmbedder_EmbedBatch(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		var req ollamaEmbedRequest
		json.NewDecoder(r.Body).Decode(&req)

		// The batch input should be an array.
		inputs, ok := req.Input.([]any)
		if !ok {
			t.Fatalf("expected array input for batch, got %T", req.Input)
		}

		vecs := make([][]float32, len(inputs))
		for i := range vecs {
			vecs[i] = make([]float32, 768)
			vecs[i][0] = float32(i) + 1.0
		}

		resp := ollamaEmbedResponse{Embeddings: vecs}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "nomic-embed-text")
	vecs, err := embedder.EmbedBatch(context.Background(), []string{"text1", "text2", "text3"})
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(vecs) != 3 {
		t.Errorf("expected 3 vectors, got %d", len(vecs))
	}
	if callCount != 1 {
		t.Errorf("expected 1 API call for batch, got %d", callCount)
	}

	// Verify each vector has distinct first element.
	for i, v := range vecs {
		expected := float32(i) + 1.0
		if v[0] != expected {
			t.Errorf("vec[%d][0] = %f, want %f", i, v[0], expected)
		}
	}
}

func TestOllamaEmbedder_EmbedBatch_Empty(t *testing.T) {
	embedder := NewOllamaEmbedder("http://unused", "model")
	vecs, err := embedder.EmbedBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("EmbedBatch(nil) failed: %v", err)
	}
	if vecs != nil {
		t.Errorf("expected nil for empty input, got %v", vecs)
	}
}

func TestOllamaEmbedder_EmbedBatch_Single(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vec := make([]float32, 768)
		vec[0] = 42.0
		resp := ollamaEmbedResponse{Embeddings: [][]float32{vec}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "model")
	vecs, err := embedder.EmbedBatch(context.Background(), []string{"single"})
	if err != nil {
		t.Fatalf("EmbedBatch(single) failed: %v", err)
	}
	if len(vecs) != 1 {
		t.Fatalf("expected 1 vector, got %d", len(vecs))
	}
	if vecs[0][0] != 42.0 {
		t.Errorf("expected 42.0, got %f", vecs[0][0])
	}
}

func TestOllamaEmbedder_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("model not found"))
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "missing-model")
	_, err := embedder.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

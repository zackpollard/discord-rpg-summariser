package embed

import "context"

// Embedder generates vector embeddings from text.
type Embedder interface {
	// Embed returns the embedding vector for a single text string.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch returns embedding vectors for multiple text strings.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

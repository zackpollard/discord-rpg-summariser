package embed

import (
	"context"
	"math"
	"os"
	"testing"
)

// TestOnnxEmbedder_Integration is a full integration test that downloads
// the model and runs real inference. Skipped unless EMBED_INTEGRATION=1.
func TestOnnxEmbedder_Integration(t *testing.T) {
	if os.Getenv("EMBED_INTEGRATION") != "1" {
		t.Skip("set EMBED_INTEGRATION=1 to run (downloads ~137MB model)")
	}

	modelDir := t.TempDir()
	embedder, err := NewOnnxEmbedder(modelDir, 4)
	if err != nil {
		t.Fatalf("NewOnnxEmbedder: %v", err)
	}
	defer embedder.Close()

	ctx := context.Background()

	t.Run("Embed", func(t *testing.T) {
		vec, err := embedder.Embed(ctx, "What is the capital of France?")
		if err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if len(vec) != hiddenSize {
			t.Errorf("expected %d-dim vector, got %d", hiddenSize, len(vec))
		}

		// Should be L2-normalised (magnitude ≈ 1.0).
		var mag float64
		for _, v := range vec {
			mag += float64(v) * float64(v)
		}
		mag = math.Sqrt(mag)
		if math.Abs(mag-1.0) > 0.01 {
			t.Errorf("expected unit vector (mag ≈ 1.0), got %.4f", mag)
		}
	})

	t.Run("EmbedBatch", func(t *testing.T) {
		texts := []string{
			"The party entered the dungeon.",
			"A fierce dragon appeared before them.",
			"The wizard cast a fireball spell.",
		}
		vecs, err := embedder.EmbedBatch(ctx, texts)
		if err != nil {
			t.Fatalf("EmbedBatch: %v", err)
		}
		if len(vecs) != 3 {
			t.Fatalf("expected 3 vectors, got %d", len(vecs))
		}
		for i, vec := range vecs {
			if len(vec) != hiddenSize {
				t.Errorf("vec[%d]: expected %d dims, got %d", i, hiddenSize, len(vec))
			}
		}

		// Vectors for similar RPG content should be more similar to each
		// other than to a completely unrelated query.
		sim01 := cosine(vecs[0], vecs[1])
		sim02 := cosine(vecs[0], vecs[2])
		t.Logf("similarity(dungeon, dragon)=%.3f similarity(dungeon, fireball)=%.3f", sim01, sim02)

		// All should have non-trivial similarity (same domain).
		if sim01 < 0.2 || sim02 < 0.2 {
			t.Errorf("expected similar domain texts to have similarity > 0.2")
		}
	})

	t.Run("EmbedBatch_Empty", func(t *testing.T) {
		vecs, err := embedder.EmbedBatch(ctx, nil)
		if err != nil {
			t.Fatalf("EmbedBatch(nil): %v", err)
		}
		if vecs != nil {
			t.Errorf("expected nil for empty input, got %v", vecs)
		}
	})
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

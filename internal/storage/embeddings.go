package storage

import (
	"context"
	"fmt"

	pgvector "github.com/pgvector/pgvector-go"
)

// EmbeddingDoc represents a document to be stored with its embedding vector.
type EmbeddingDoc struct {
	CampaignID int64
	DocType    string // "summary", "transcript_chunk", "entity", "quest"
	DocID      int64
	SessionID  *int64  // nil if not session-scoped
	Title      string
	Content    string
	Embedding  []float32
}

// EmbeddingSearchResult is a row returned by similarity search.
type EmbeddingSearchResult struct {
	ID         int64   `json:"id"`
	DocType    string  `json:"doc_type"`
	DocID      int64   `json:"doc_id"`
	SessionID  *int64  `json:"session_id,omitempty"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Similarity float64 `json:"similarity"`
}

// UpsertEmbedding inserts or updates an embedding document. The unique
// constraint on (campaign_id, doc_type, doc_id) is used for conflict handling.
func (s *Store) UpsertEmbedding(ctx context.Context, doc EmbeddingDoc) error {
	vec := pgvector.NewVector(doc.Embedding)
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO embeddings (campaign_id, doc_type, doc_id, session_id, title, content, embedding)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (campaign_id, doc_type, doc_id) DO UPDATE SET
		   session_id = EXCLUDED.session_id,
		   title = EXCLUDED.title,
		   content = EXCLUDED.content,
		   embedding = EXCLUDED.embedding,
		   created_at = NOW()`,
		doc.CampaignID, doc.DocType, doc.DocID, doc.SessionID, doc.Title, doc.Content, vec,
	)
	if err != nil {
		return fmt.Errorf("upsert embedding: %w", err)
	}
	return nil
}

// DeleteEmbeddingsForSession removes all embeddings associated with a session.
func (s *Store) DeleteEmbeddingsForSession(ctx context.Context, sessionID int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM embeddings WHERE session_id = $1`, sessionID)
	if err != nil {
		return fmt.Errorf("delete embeddings for session: %w", err)
	}
	return nil
}

// DeleteEmbeddingsByDocType removes embeddings by campaign, doc type, and doc ID.
func (s *Store) DeleteEmbeddingsByDocType(ctx context.Context, campaignID int64, docType string, docID int64) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM embeddings WHERE campaign_id = $1 AND doc_type = $2 AND doc_id = $3`,
		campaignID, docType, docID,
	)
	if err != nil {
		return fmt.Errorf("delete embeddings by doc type: %w", err)
	}
	return nil
}

// SearchSimilar performs a cosine similarity search against the embeddings
// table for a given campaign. Results are ordered by descending similarity.
func (s *Store) SearchSimilar(ctx context.Context, campaignID int64, queryVec []float32, limit int) ([]EmbeddingSearchResult, error) {
	vec := pgvector.NewVector(queryVec)
	rows, err := s.Pool.Query(ctx,
		`SELECT id, doc_type, doc_id, session_id, title, content,
		        1 - (embedding <=> $2::vector) AS similarity
		 FROM embeddings
		 WHERE campaign_id = $1
		 ORDER BY embedding <=> $2::vector
		 LIMIT $3`,
		campaignID, vec, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search similar embeddings: %w", err)
	}
	defer rows.Close()

	var results []EmbeddingSearchResult
	for rows.Next() {
		var r EmbeddingSearchResult
		if err := rows.Scan(&r.ID, &r.DocType, &r.DocID, &r.SessionID, &r.Title, &r.Content, &r.Similarity); err != nil {
			return nil, fmt.Errorf("scan embedding result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

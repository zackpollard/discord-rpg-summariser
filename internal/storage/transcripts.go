package storage

import (
	"context"
	"fmt"
	"time"
)

// TranscriptSearchResult holds a single full-text search hit across transcript segments.
type TranscriptSearchResult struct {
	SegmentID int64   `json:"segment_id"`
	SessionID int64   `json:"session_id"`
	UserID    string  `json:"user_id"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
	Text      string  `json:"text"`
	Headline  string  `json:"headline"` // ts_headline with <mark> tags
	SessionAt time.Time `json:"session_started_at"`
}

// SearchTranscripts performs full-text search across transcript segments for a given campaign.
// It returns matching results ordered by relevance, plus the total count for pagination.
func (s *Store) SearchTranscripts(ctx context.Context, campaignID int64, query string, limit, offset int) ([]TranscriptSearchResult, int, error) {
	if query == "" {
		return []TranscriptSearchResult{}, 0, nil
	}

	// Count total matches for pagination.
	var total int
	err := s.Pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM transcript_segments ts
		 JOIN sessions s ON s.id = ts.session_id
		 WHERE s.campaign_id = $1
		   AND ts.tsv @@ websearch_to_tsquery('english', $2)`,
		campaignID, query,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count transcript search results: %w", err)
	}

	if total == 0 {
		return []TranscriptSearchResult{}, 0, nil
	}

	rows, err := s.Pool.Query(ctx,
		`SELECT ts.id, ts.session_id, ts.user_id, ts.start_time, ts.end_time, ts.text,
		        ts_headline('english', ts.text, websearch_to_tsquery('english', $2),
		            'StartSel=<mark>, StopSel=</mark>, MaxFragments=2, MaxWords=30, MinWords=10') AS headline,
		        s.started_at
		 FROM transcript_segments ts
		 JOIN sessions s ON s.id = ts.session_id
		 WHERE s.campaign_id = $1
		   AND ts.tsv @@ websearch_to_tsquery('english', $2)
		 ORDER BY ts_rank(ts.tsv, websearch_to_tsquery('english', $2)) DESC
		 LIMIT $3 OFFSET $4`,
		campaignID, query, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("search transcripts: %w", err)
	}
	defer rows.Close()

	var results []TranscriptSearchResult
	for rows.Next() {
		var r TranscriptSearchResult
		if err := rows.Scan(&r.SegmentID, &r.SessionID, &r.UserID,
			&r.StartTime, &r.EndTime, &r.Text, &r.Headline, &r.SessionAt); err != nil {
			return nil, 0, fmt.Errorf("scan transcript search result: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate transcript search results: %w", err)
	}

	return results, total, nil
}

type TranscriptSegment struct {
	ID        int64
	SessionID int64
	UserID    string
	StartTime float64
	EndTime   float64
	Text      string
	CreatedAt time.Time
}

func (s *Store) InsertSegments(ctx context.Context, segments []TranscriptSegment) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, seg := range segments {
		_, err := tx.Exec(ctx,
			`INSERT INTO transcript_segments (session_id, user_id, start_time, end_time, text)
			 VALUES ($1, $2, $3, $4, $5)`,
			seg.SessionID, seg.UserID, seg.StartTime, seg.EndTime, seg.Text,
		)
		if err != nil {
			return fmt.Errorf("insert segment: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) GetTranscript(ctx context.Context, sessionID int64) ([]TranscriptSegment, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, session_id, user_id, start_time, end_time, text, created_at
		 FROM transcript_segments WHERE session_id = $1 ORDER BY start_time`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query transcript: %w", err)
	}
	defer rows.Close()

	var segments []TranscriptSegment
	for rows.Next() {
		var seg TranscriptSegment
		err := rows.Scan(&seg.ID, &seg.SessionID, &seg.UserID,
			&seg.StartTime, &seg.EndTime, &seg.Text, &seg.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan segment: %w", err)
		}
		segments = append(segments, seg)
	}
	return segments, rows.Err()
}

// DeleteTranscriptSegments removes all transcript segments for a session.
func (s *Store) DeleteTranscriptSegments(ctx context.Context, sessionID int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM transcript_segments WHERE session_id = $1`, sessionID)
	return err
}

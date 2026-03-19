package storage

import (
	"context"
	"fmt"
	"time"
)

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

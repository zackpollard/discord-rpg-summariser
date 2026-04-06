package storage

import (
	"context"
	"fmt"
	"time"
)

type SessionQuote struct {
	ID        int64
	SessionID int64
	Speaker   string
	Text      string
	StartTime float64
	Tone      *string
	CreatedAt time.Time
}

func (s *Store) InsertQuotes(ctx context.Context, quotes []SessionQuote) error {
	if len(quotes) == 0 {
		return nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, q := range quotes {
		_, err := tx.Exec(ctx,
			`INSERT INTO session_quotes (session_id, speaker, text, start_time, tone)
			 VALUES ($1, $2, $3, $4, $5)`,
			q.SessionID, q.Speaker, q.Text, q.StartTime, q.Tone,
		)
		if err != nil {
			return fmt.Errorf("insert quote: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) GetQuotes(ctx context.Context, sessionID int64) ([]SessionQuote, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, session_id, speaker, text, start_time, tone, created_at
		 FROM session_quotes WHERE session_id = $1 ORDER BY start_time`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("get quotes: %w", err)
	}
	defer rows.Close()

	var quotes []SessionQuote
	for rows.Next() {
		var q SessionQuote
		if err := rows.Scan(&q.ID, &q.SessionID, &q.Speaker, &q.Text, &q.StartTime, &q.Tone, &q.CreatedAt); err != nil {
			return nil, err
		}
		quotes = append(quotes, q)
	}
	return quotes, rows.Err()
}

package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// LLMLog represents a single LLM request/response pair.
type LLMLog struct {
	ID         int64
	SessionID  *int64
	Operation  string
	Prompt     string
	Response   string
	Error      *string
	DurationMS int
	CreatedAt  time.Time
}

func (s *Store) InsertLLMLog(ctx context.Context, log LLMLog) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO llm_logs (session_id, operation, prompt, response, error, duration_ms)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		log.SessionID, log.Operation, log.Prompt, log.Response, log.Error, log.DurationMS,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert llm log: %w", err)
	}
	return id, nil
}

func (s *Store) GetLLMLogsForSession(ctx context.Context, sessionID int64) ([]LLMLog, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, session_id, operation, prompt, response, error, duration_ms, created_at
		 FROM llm_logs WHERE session_id = $1 ORDER BY created_at ASC`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("get llm logs: %w", err)
	}
	defer rows.Close()

	var logs []LLMLog
	for rows.Next() {
		l, err := scanLLMLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, *l)
	}
	return logs, rows.Err()
}

func scanLLMLog(rows pgx.Rows) (*LLMLog, error) {
	var l LLMLog
	err := rows.Scan(&l.ID, &l.SessionID, &l.Operation, &l.Prompt, &l.Response, &l.Error, &l.DurationMS, &l.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

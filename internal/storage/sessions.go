package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Session struct {
	ID        int64
	GuildID   string
	ChannelID string
	StartedAt time.Time
	EndedAt   *time.Time
	Status    string
	AudioDir  string
	Summary   *string
	KeyEvents []string
	CreatedAt time.Time
}

func (s *Store) CreateSession(ctx context.Context, guildID, channelID, audioDir string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO sessions (guild_id, channel_id, started_at, audio_dir, status)
		 VALUES ($1, $2, $3, $4, 'recording') RETURNING id`,
		guildID, channelID, time.Now().UTC(), audioDir,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create session: %w", err)
	}
	return id, nil
}

func (s *Store) GetSession(ctx context.Context, id int64) (*Session, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, guild_id, channel_id, started_at, ended_at, status, audio_dir, summary, key_events, created_at
		 FROM sessions WHERE id = $1`, id,
	)
	return scanSession(row)
}

func (s *Store) ListSessions(ctx context.Context, guildID string, limit, offset int) ([]Session, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, guild_id, channel_id, started_at, ended_at, status, audio_dir, summary, key_events, created_at
		 FROM sessions WHERE guild_id = $1 ORDER BY started_at DESC LIMIT $2 OFFSET $3`,
		guildID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		sess, err := scanSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *sess)
	}
	return sessions, rows.Err()
}

func (s *Store) UpdateSessionStatus(ctx context.Context, id int64, status string) error {
	_, err := s.Pool.Exec(ctx, "UPDATE sessions SET status = $1 WHERE id = $2", status, id)
	return err
}

func (s *Store) EndSession(ctx context.Context, id int64) error {
	_, err := s.Pool.Exec(ctx,
		"UPDATE sessions SET ended_at = $1, status = 'transcribing' WHERE id = $2",
		time.Now().UTC(), id,
	)
	return err
}

func (s *Store) UpdateSessionSummary(ctx context.Context, id int64, summary string, keyEvents []string) error {
	eventsJSON, err := json.Marshal(keyEvents)
	if err != nil {
		return fmt.Errorf("marshal key events: %w", err)
	}
	_, err = s.Pool.Exec(ctx,
		"UPDATE sessions SET summary = $1, key_events = $2, status = 'complete' WHERE id = $3",
		summary, string(eventsJSON), id,
	)
	return err
}

// CleanupStaleSessions marks any sessions stuck in non-terminal states as
// 'failed'. This handles the case where the bot was killed mid-recording.
func (s *Store) CleanupStaleSessions(ctx context.Context) (int64, error) {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE sessions SET status = 'failed', ended_at = NOW()
		 WHERE status IN ('recording', 'transcribing', 'summarising')`)
	if err != nil {
		return 0, fmt.Errorf("cleanup stale sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (s *Store) GetActiveSession(ctx context.Context, guildID string) (*Session, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, guild_id, channel_id, started_at, ended_at, status, audio_dir, summary, key_events, created_at
		 FROM sessions WHERE guild_id = $1 AND status = 'recording' LIMIT 1`, guildID,
	)
	sess, err := scanSession(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return sess, err
}

func scanSession(row pgx.Row) (*Session, error) {
	var sess Session
	var keyEventsJSON []byte

	err := row.Scan(
		&sess.ID, &sess.GuildID, &sess.ChannelID, &sess.StartedAt,
		&sess.EndedAt, &sess.Status, &sess.AudioDir, &sess.Summary, &keyEventsJSON, &sess.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if keyEventsJSON != nil {
		json.Unmarshal(keyEventsJSON, &sess.KeyEvents)
	}

	return &sess, nil
}

func scanSessionRows(rows pgx.Rows) (*Session, error) {
	var sess Session
	var keyEventsJSON []byte

	err := rows.Scan(
		&sess.ID, &sess.GuildID, &sess.ChannelID, &sess.StartedAt,
		&sess.EndedAt, &sess.Status, &sess.AudioDir, &sess.Summary, &keyEventsJSON, &sess.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if keyEventsJSON != nil {
		json.Unmarshal(keyEventsJSON, &sess.KeyEvents)
	}

	return &sess, nil
}

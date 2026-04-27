package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Session struct {
	ID         int64
	GuildID    string
	CampaignID int64
	ChannelID  string
	StartedAt  time.Time
	EndedAt    *time.Time
	Status     string
	AudioDir   string
	Summary    *string
	KeyEvents  []string
	Title      *string
	CreatedAt  time.Time
}

const sessionColumns = `id, guild_id, campaign_id, channel_id, started_at, ended_at, status, audio_dir, summary, key_events, title, created_at`

func (s *Store) CreateSession(ctx context.Context, guildID string, campaignID int64, channelID, audioDir string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO sessions (guild_id, campaign_id, channel_id, started_at, audio_dir, status)
		 VALUES ($1, $2, $3, $4, $5, 'recording') RETURNING id`,
		guildID, campaignID, channelID, time.Now().UTC(), audioDir,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create session: %w", err)
	}
	return id, nil
}

func (s *Store) GetSession(ctx context.Context, id int64) (*Session, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT `+sessionColumns+` FROM sessions WHERE id = $1`, id,
	)
	return scanSession(row)
}

func (s *Store) ListSessions(ctx context.Context, guildID string, campaignID int64, limit, offset int) ([]Session, error) {
	var rows pgx.Rows
	var err error
	if campaignID > 0 {
		rows, err = s.Pool.Query(ctx,
			`SELECT `+sessionColumns+` FROM sessions WHERE guild_id = $1 AND campaign_id = $2 ORDER BY started_at DESC LIMIT $3 OFFSET $4`,
			guildID, campaignID, limit, offset,
		)
	} else {
		rows, err = s.Pool.Query(ctx,
			`SELECT `+sessionColumns+` FROM sessions WHERE guild_id = $1 ORDER BY started_at DESC LIMIT $2 OFFSET $3`,
			guildID, limit, offset,
		)
	}
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

// GetLatestCompleteSessions returns the N most recent sessions with status
// 'complete' and a non-null summary for the given campaign, ordered
// chronologically (oldest first).
func (s *Store) GetLatestCompleteSessions(ctx context.Context, campaignID int64, n int) ([]Session, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+sessionColumns+`
		 FROM sessions
		 WHERE campaign_id = $1 AND status = 'complete' AND summary IS NOT NULL
		 ORDER BY started_at DESC
		 LIMIT $2`,
		campaignID, n,
	)
	if err != nil {
		return nil, fmt.Errorf("get latest complete sessions: %w", err)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to chronological order (oldest first).
	for i, j := 0, len(sessions)-1; i < j; i, j = i+1, j-1 {
		sessions[i], sessions[j] = sessions[j], sessions[i]
	}
	return sessions, nil
}

// CleanupStaleSessions marks sessions left in an active state (recording /
// transcribing / summarising) from a previous run as failed. Preserves
// ended_at if it was already set so durations stay accurate — only stamps
// NOW() for sessions that genuinely never ended (stuck mid-recording).
func (s *Store) CleanupStaleSessions(ctx context.Context) (int64, error) {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE sessions SET status = 'failed', ended_at = COALESCE(ended_at, NOW())
		 WHERE status IN ('recording', 'transcribing', 'summarising')`)
	if err != nil {
		return 0, fmt.Errorf("cleanup stale sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

// SessionForBackfill is a thin record used by RecalculateSessionEndTimes to
// pull the candidate set without exposing the full Session type.
type SessionForBackfill struct {
	ID        int64
	StartedAt time.Time
	EndedAt   *time.Time
	AudioDir  string
}

// ListSessionsForEndTimeBackfill returns sessions whose ended_at was likely
// stamped by a bot restart rather than the actual recording end (i.e.
// duration > maxRealDuration). The caller derives a real end-time from the
// audio directory's latest file mtime and updates the row.
func (s *Store) ListSessionsForEndTimeBackfill(ctx context.Context, maxRealDuration time.Duration) ([]SessionForBackfill, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, started_at, ended_at, audio_dir
		FROM sessions
		WHERE ended_at IS NOT NULL
		  AND audio_dir <> ''
		  AND EXTRACT(EPOCH FROM (ended_at - started_at)) > $1
	`, maxRealDuration.Seconds())
	if err != nil {
		return nil, fmt.Errorf("list sessions for backfill: %w", err)
	}
	defer rows.Close()
	var out []SessionForBackfill
	for rows.Next() {
		var s SessionForBackfill
		if err := rows.Scan(&s.ID, &s.StartedAt, &s.EndedAt, &s.AudioDir); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetSessionEndedAt updates only the ended_at column for a session.
func (s *Store) SetSessionEndedAt(ctx context.Context, id int64, endedAt time.Time) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE sessions SET ended_at = $1 WHERE id = $2`,
		endedAt.UTC(), id,
	)
	return err
}

// DeleteSession permanently removes a session and all associated data.
// Child rows (transcript_segments, telegram_messages, entity_notes,
// quest_updates, entity_references, combat_encounters, embeddings) are
// removed via ON DELETE CASCADE. Audio files are NOT removed here.
func (s *Store) DeleteSession(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) GetActiveSession(ctx context.Context, guildID string) (*Session, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT `+sessionColumns+` FROM sessions WHERE guild_id = $1 AND status = 'recording' LIMIT 1`, guildID,
	)
	sess, err := scanSession(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return sess, err
}

func (s *Store) UpdateSessionTitle(ctx context.Context, id int64, title string) error {
	_, err := s.Pool.Exec(ctx, "UPDATE sessions SET title = $1 WHERE id = $2", title, id)
	return err
}

func scanSession(row pgx.Row) (*Session, error) {
	var sess Session
	var keyEventsJSON []byte

	err := row.Scan(
		&sess.ID, &sess.GuildID, &sess.CampaignID, &sess.ChannelID, &sess.StartedAt,
		&sess.EndedAt, &sess.Status, &sess.AudioDir, &sess.Summary, &keyEventsJSON, &sess.Title, &sess.CreatedAt,
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
		&sess.ID, &sess.GuildID, &sess.CampaignID, &sess.ChannelID, &sess.StartedAt,
		&sess.EndedAt, &sess.Status, &sess.AudioDir, &sess.Summary, &keyEventsJSON, &sess.Title, &sess.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if keyEventsJSON != nil {
		json.Unmarshal(keyEventsJSON, &sess.KeyEvents)
	}
	return &sess, nil
}

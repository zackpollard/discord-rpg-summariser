package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type SoundboardClip struct {
	ID         int64
	CampaignID int64
	SessionID  *int64
	Name       string
	AudioPath  string
	StartTime  float64
	EndTime    float64
	UserIDs    []string
	CreatedAt  time.Time
}

func (s *Store) InsertSoundboardClip(ctx context.Context, clip SoundboardClip) (int64, error) {
	userIDsJSON, _ := json.Marshal(clip.UserIDs)
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO soundboard_clips (campaign_id, session_id, name, audio_path, start_time, end_time, user_ids)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		clip.CampaignID, clip.SessionID, clip.Name, clip.AudioPath,
		clip.StartTime, clip.EndTime, string(userIDsJSON),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert soundboard clip: %w", err)
	}
	return id, nil
}

func (s *Store) ListSoundboardClips(ctx context.Context, campaignID int64) ([]SoundboardClip, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, campaign_id, session_id, name, audio_path, start_time, end_time, user_ids, created_at
		 FROM soundboard_clips WHERE campaign_id = $1 ORDER BY created_at DESC`, campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("list soundboard clips: %w", err)
	}
	defer rows.Close()

	var clips []SoundboardClip
	for rows.Next() {
		c, err := scanClip(rows)
		if err != nil {
			return nil, err
		}
		clips = append(clips, *c)
	}
	return clips, rows.Err()
}

func (s *Store) GetSoundboardClip(ctx context.Context, id int64) (*SoundboardClip, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, campaign_id, session_id, name, audio_path, start_time, end_time, user_ids, created_at
		 FROM soundboard_clips WHERE id = $1`, id,
	)
	if err != nil {
		return nil, fmt.Errorf("get soundboard clip: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, pgx.ErrNoRows
	}
	return scanClip(rows)
}

func (s *Store) DeleteSoundboardClip(ctx context.Context, id int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM soundboard_clips WHERE id = $1`, id)
	return err
}

func scanClip(rows pgx.Rows) (*SoundboardClip, error) {
	var c SoundboardClip
	var userIDsJSON []byte
	err := rows.Scan(&c.ID, &c.CampaignID, &c.SessionID, &c.Name, &c.AudioPath,
		&c.StartTime, &c.EndTime, &userIDsJSON, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	if userIDsJSON != nil {
		json.Unmarshal(userIDsJSON, &c.UserIDs)
	}
	return &c, nil
}

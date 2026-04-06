package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type VoiceProfile struct {
	ID         int64
	CampaignID int64
	Name       string
	AudioPath  string
	Transcript string
	CreatedAt  time.Time
}

func (s *Store) InsertVoiceProfile(ctx context.Context, campaignID int64, name, audioPath, transcript string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO voice_profiles (campaign_id, name, audio_path, transcript)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		campaignID, name, audioPath, transcript,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert voice profile: %w", err)
	}
	return id, nil
}

func (s *Store) GetVoiceProfiles(ctx context.Context, campaignID int64) ([]VoiceProfile, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, campaign_id, name, audio_path, transcript, created_at
		 FROM voice_profiles WHERE campaign_id = $1 ORDER BY name`, campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("get voice profiles: %w", err)
	}
	defer rows.Close()

	var profiles []VoiceProfile
	for rows.Next() {
		var p VoiceProfile
		if err := rows.Scan(&p.ID, &p.CampaignID, &p.Name, &p.AudioPath, &p.Transcript, &p.CreatedAt); err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (s *Store) GetVoiceProfile(ctx context.Context, id int64) (*VoiceProfile, error) {
	var p VoiceProfile
	err := s.Pool.QueryRow(ctx,
		`SELECT id, campaign_id, name, audio_path, transcript, created_at
		 FROM voice_profiles WHERE id = $1`, id,
	).Scan(&p.ID, &p.CampaignID, &p.Name, &p.AudioPath, &p.Transcript, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get voice profile: %w", err)
	}
	return &p, nil
}

func (s *Store) DeleteVoiceProfile(ctx context.Context, id int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM voice_profiles WHERE id = $1`, id)
	return err
}

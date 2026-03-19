package storage

import (
	"context"
	"fmt"
	"time"
)

// SharedMic represents a shared microphone configuration where one Discord
// user's audio contains two speakers.
type SharedMic struct {
	ID             int64
	CampaignID     int64
	DiscordUserID  string
	SpeakerAUserID string // typically the DM
	SpeakerBUserID string // the partner (synthetic ID)
	CreatedAt      time.Time
}

// SyntheticPartnerID generates a synthetic user ID for the partner sharing a mic.
func SyntheticPartnerID(discordUserID string) string {
	return "shared:" + discordUserID + ":partner"
}

func (s *Store) SetSharedMic(ctx context.Context, campaignID int64, discordUserID, speakerAUserID, speakerBUserID string) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO shared_mics (campaign_id, discord_user_id, speaker_a_user_id, speaker_b_user_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (campaign_id, discord_user_id)
		 DO UPDATE SET speaker_a_user_id = $3, speaker_b_user_id = $4`,
		campaignID, discordUserID, speakerAUserID, speakerBUserID,
	)
	return err
}

func (s *Store) GetSharedMics(ctx context.Context, campaignID int64) ([]SharedMic, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, campaign_id, discord_user_id, speaker_a_user_id, speaker_b_user_id, created_at
		 FROM shared_mics WHERE campaign_id = $1`, campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("query shared mics: %w", err)
	}
	defer rows.Close()

	var mics []SharedMic
	for rows.Next() {
		var m SharedMic
		if err := rows.Scan(&m.ID, &m.CampaignID, &m.DiscordUserID, &m.SpeakerAUserID, &m.SpeakerBUserID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan shared mic: %w", err)
		}
		mics = append(mics, m)
	}
	return mics, rows.Err()
}

func (s *Store) DeleteSharedMic(ctx context.Context, campaignID int64, discordUserID string) error {
	_, err := s.Pool.Exec(ctx,
		"DELETE FROM shared_mics WHERE campaign_id = $1 AND discord_user_id = $2",
		campaignID, discordUserID,
	)
	return err
}

package storage

import (
	"context"
	"fmt"
	"time"
)

// SharedMic represents a shared microphone configuration where one Discord
// user's audio contains two speakers.
type SharedMic struct {
	ID            int64
	CampaignID    int64
	DiscordUserID string // mic owner (the Discord user recording audio)
	PartnerUserID string // the other person (real Discord user ID or synthetic ID)
	CreatedAt     time.Time
}

// SyntheticPartnerID generates a synthetic user ID for a partner sharing a mic
// who is not a Discord user.
func SyntheticPartnerID(discordUserID string) string {
	return "shared:" + discordUserID + ":partner"
}

func (s *Store) SetSharedMic(ctx context.Context, campaignID int64, discordUserID, partnerUserID string) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO shared_mics (campaign_id, discord_user_id, partner_user_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (campaign_id, discord_user_id)
		 DO UPDATE SET partner_user_id = $3`,
		campaignID, discordUserID, partnerUserID,
	)
	return err
}

func (s *Store) GetSharedMics(ctx context.Context, campaignID int64) ([]SharedMic, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, campaign_id, discord_user_id, partner_user_id, created_at
		 FROM shared_mics WHERE campaign_id = $1`, campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("query shared mics: %w", err)
	}
	defer rows.Close()

	var mics []SharedMic
	for rows.Next() {
		var m SharedMic
		if err := rows.Scan(&m.ID, &m.CampaignID, &m.DiscordUserID, &m.PartnerUserID, &m.CreatedAt); err != nil {
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

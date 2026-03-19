package storage

import (
	"context"
	"fmt"
	"time"
)

// SpeakerEnrollment stores a voice embedding for a user within a campaign,
// used to identify speakers on shared microphones.
type SpeakerEnrollment struct {
	ID         int64
	CampaignID int64
	UserID     string
	Embedding  []float32
	UpdatedAt  time.Time
}

// UpsertSpeakerEnrollment inserts or updates a voice embedding for a user.
func (s *Store) UpsertSpeakerEnrollment(ctx context.Context, campaignID int64, userID string, embedding []float32) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO speaker_enrollments (campaign_id, user_id, embedding, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (campaign_id, user_id)
		 DO UPDATE SET embedding = $3, updated_at = NOW()`,
		campaignID, userID, embedding,
	)
	return err
}

// GetSpeakerEnrollment returns the enrollment for a specific user, or nil if not found.
func (s *Store) GetSpeakerEnrollment(ctx context.Context, campaignID int64, userID string) (*SpeakerEnrollment, error) {
	var e SpeakerEnrollment
	err := s.Pool.QueryRow(ctx,
		`SELECT id, campaign_id, user_id, embedding, updated_at
		 FROM speaker_enrollments
		 WHERE campaign_id = $1 AND user_id = $2`,
		campaignID, userID,
	).Scan(&e.ID, &e.CampaignID, &e.UserID, &e.Embedding, &e.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get speaker enrollment: %w", err)
	}
	return &e, nil
}

// GetSpeakerEnrollments returns all enrollments for a campaign.
func (s *Store) GetSpeakerEnrollments(ctx context.Context, campaignID int64) ([]SpeakerEnrollment, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, campaign_id, user_id, embedding, updated_at
		 FROM speaker_enrollments WHERE campaign_id = $1`, campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("query speaker enrollments: %w", err)
	}
	defer rows.Close()

	var enrollments []SpeakerEnrollment
	for rows.Next() {
		var e SpeakerEnrollment
		if err := rows.Scan(&e.ID, &e.CampaignID, &e.UserID, &e.Embedding, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan speaker enrollment: %w", err)
		}
		enrollments = append(enrollments, e)
	}
	return enrollments, rows.Err()
}

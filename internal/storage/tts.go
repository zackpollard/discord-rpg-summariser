package storage

import (
	"context"
	"fmt"
)

// GetUsersWithAudio returns distinct user IDs who have transcript segments
// in sessions that also have audio directories, for the given campaign.
func (s *Store) GetUsersWithAudio(ctx context.Context, campaignID int64) ([]string, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT DISTINCT ts.user_id
		 FROM transcript_segments ts
		 JOIN sessions s ON s.id = ts.session_id
		 WHERE s.campaign_id = $1 AND s.audio_dir != '' AND s.status = 'complete'`,
		campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("get users with audio: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("scan user id: %w", err)
		}
		userIDs = append(userIDs, uid)
	}
	return userIDs, rows.Err()
}

// GetUserSessionWithAudio returns the most recent session for the given
// campaign where the user has transcript segments and audio exists.
func (s *Store) GetUserSessionWithAudio(ctx context.Context, campaignID int64, userID string) (*Session, error) {
	sessions, err := s.GetUserSessionsWithAudio(ctx, campaignID, userID, 1)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	return &sessions[0], nil
}

// GetUserSessionsWithAudio returns the most recent sessions (up to limit) for
// the given campaign where the user has transcript segments and audio exists.
func (s *Store) GetUserSessionsWithAudio(ctx context.Context, campaignID int64, userID string, limit int) ([]Session, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+sessionColumns+` FROM sessions s
		 WHERE s.campaign_id = $1 AND s.audio_dir != '' AND s.status = 'complete'
		   AND EXISTS (SELECT 1 FROM transcript_segments ts WHERE ts.session_id = s.id AND ts.user_id = $2)
		 ORDER BY s.started_at DESC LIMIT $3`,
		campaignID, userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get user sessions with audio: %w", err)
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

package storage

import (
	"context"
	"fmt"
	"time"
)

type TimelineEvent struct {
	Type      string    `json:"type"` // session, entity, quest_new, quest_completed, quest_failed
	Timestamp time.Time `json:"timestamp"`
	Title     string    `json:"title"`
	Detail    string    `json:"detail"`
	SessionID *int64    `json:"session_id,omitempty"`
	EntityID  *int64    `json:"entity_id,omitempty"`
	QuestID   *int64    `json:"quest_id,omitempty"`
}

func truncate(s string, maxChars int) string {
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	return string(r[:maxChars])
}

func (s *Store) GetCampaignTimeline(ctx context.Context, campaignID int64, limit, offset int) ([]TimelineEvent, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT type, timestamp, title, detail, session_id, entity_id, quest_id FROM (
			SELECT 'session' as type, s.started_at as timestamp,
				'Session #' || s.id as title,
				COALESCE(s.summary, s.status, '') as detail,
				s.id as session_id, NULL::BIGINT as entity_id, NULL::BIGINT as quest_id
			FROM sessions s WHERE s.campaign_id = $1 AND s.status != 'failed'

			UNION ALL

			SELECT 'entity' as type, e.created_at as timestamp,
				e.name as title,
				COALESCE(e.type || ': ' || e.description, '') as detail,
				NULL::BIGINT, e.id, NULL::BIGINT
			FROM entities e WHERE e.campaign_id = $1

			UNION ALL

			SELECT 'quest_new' as type, q.created_at as timestamp,
				q.name as title,
				COALESCE(CASE WHEN q.giver != '' THEN 'From ' || q.giver || ': ' ELSE '' END || q.description, '') as detail,
				NULL::BIGINT, NULL::BIGINT, q.id
			FROM quests q WHERE q.campaign_id = $1

			UNION ALL

			SELECT 'quest_' || q.status as type, q.updated_at as timestamp,
				q.name as title,
				'Quest ' || q.status as detail,
				NULL::BIGINT, NULL::BIGINT, q.id
			FROM quests q WHERE q.campaign_id = $1 AND q.status IN ('completed', 'failed')
				AND q.updated_at != q.created_at
		) timeline
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3`, campaignID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get timeline: %w", err)
	}
	defer rows.Close()

	var events []TimelineEvent
	for rows.Next() {
		var e TimelineEvent
		if err := rows.Scan(&e.Type, &e.Timestamp, &e.Title, &e.Detail, &e.SessionID, &e.EntityID, &e.QuestID); err != nil {
			return nil, err
		}
		e.Detail = truncate(e.Detail, 200)
		events = append(events, e)
	}
	return events, rows.Err()
}

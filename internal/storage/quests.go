package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Quest struct {
	ID          int64
	CampaignID  int64
	Name        string
	Description string
	Status      string
	Giver       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type QuestUpdate struct {
	ID        int64
	QuestID   int64
	SessionID int64
	Content   string
	NewStatus *string
	CreatedAt time.Time
}

func (s *Store) UpsertQuest(ctx context.Context, campaignID int64, name, description, status, giver string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO quests (campaign_id, name, description, status, giver)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (campaign_id, name) DO UPDATE SET
		   description = CASE WHEN $3 = '' THEN quests.description ELSE $3 END,
		   status = CASE WHEN $4 = '' THEN quests.status ELSE $4 END,
		   giver = CASE WHEN $5 = '' THEN quests.giver ELSE $5 END,
		   updated_at = NOW()
		 RETURNING id`,
		campaignID, name, description, status, giver,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert quest: %w", err)
	}
	return id, nil
}

func (s *Store) GetQuest(ctx context.Context, id int64) (*Quest, error) {
	var q Quest
	err := s.Pool.QueryRow(ctx,
		`SELECT id, campaign_id, name, description, status, giver, created_at, updated_at
		 FROM quests WHERE id = $1`, id,
	).Scan(&q.ID, &q.CampaignID, &q.Name, &q.Description, &q.Status, &q.Giver, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (s *Store) ListQuests(ctx context.Context, campaignID int64, statusFilter string) ([]Quest, error) {
	var rows pgx.Rows
	var err error
	if statusFilter != "" {
		rows, err = s.Pool.Query(ctx,
			`SELECT id, campaign_id, name, description, status, giver, created_at, updated_at
			 FROM quests WHERE campaign_id = $1 AND status = $2 ORDER BY updated_at DESC`, campaignID, statusFilter)
	} else {
		rows, err = s.Pool.Query(ctx,
			`SELECT id, campaign_id, name, description, status, giver, created_at, updated_at
			 FROM quests WHERE campaign_id = $1 ORDER BY updated_at DESC`, campaignID)
	}
	if err != nil {
		return nil, fmt.Errorf("list quests: %w", err)
	}
	defer rows.Close()

	var quests []Quest
	for rows.Next() {
		var q Quest
		if err := rows.Scan(&q.ID, &q.CampaignID, &q.Name, &q.Description, &q.Status, &q.Giver, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		quests = append(quests, q)
	}
	return quests, rows.Err()
}

func (s *Store) UpdateQuestStatus(ctx context.Context, questID int64, status string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE quests SET status = $1, updated_at = NOW() WHERE id = $2`, status, questID)
	return err
}

func (s *Store) AddQuestUpdate(ctx context.Context, questID, sessionID int64, content string, newStatus *string) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO quest_updates (quest_id, session_id, content, new_status) VALUES ($1, $2, $3, $4)`,
		questID, sessionID, content, newStatus)
	return err
}

func (s *Store) GetQuestUpdates(ctx context.Context, questID int64) ([]QuestUpdate, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, quest_id, session_id, content, new_status, created_at
		 FROM quest_updates WHERE quest_id = $1 ORDER BY created_at`, questID)
	if err != nil {
		return nil, fmt.Errorf("get quest updates: %w", err)
	}
	defer rows.Close()

	var updates []QuestUpdate
	for rows.Next() {
		var u QuestUpdate
		if err := rows.Scan(&u.ID, &u.QuestID, &u.SessionID, &u.Content, &u.NewStatus, &u.CreatedAt); err != nil {
			return nil, err
		}
		updates = append(updates, u)
	}
	return updates, rows.Err()
}

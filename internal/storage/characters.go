package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type CharacterMapping struct {
	UserID        string
	GuildID       string
	CampaignID    int64
	CharacterName string
	UpdatedAt     time.Time
}

func (s *Store) SetCharacterMapping(ctx context.Context, m CharacterMapping) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO character_mappings (user_id, guild_id, campaign_id, character_name, updated_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, campaign_id) DO UPDATE SET character_name = $4, updated_at = $5`,
		m.UserID, m.GuildID, m.CampaignID, m.CharacterName, time.Now().UTC(),
	)
	return err
}

func (s *Store) GetCharacterMappings(ctx context.Context, campaignID int64) ([]CharacterMapping, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT user_id, guild_id, campaign_id, character_name, updated_at
		 FROM character_mappings WHERE campaign_id = $1`, campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("query character mappings: %w", err)
	}
	defer rows.Close()

	var mappings []CharacterMapping
	for rows.Next() {
		var m CharacterMapping
		if err := rows.Scan(&m.UserID, &m.GuildID, &m.CampaignID, &m.CharacterName, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan mapping: %w", err)
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

func (s *Store) GetCharacterName(ctx context.Context, userID string, campaignID int64) (string, error) {
	var name string
	err := s.Pool.QueryRow(ctx,
		"SELECT character_name FROM character_mappings WHERE user_id = $1 AND campaign_id = $2",
		userID, campaignID,
	).Scan(&name)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return name, err
}

func (s *Store) DeleteCharacterMapping(ctx context.Context, userID string, campaignID int64) error {
	_, err := s.Pool.Exec(ctx,
		"DELETE FROM character_mappings WHERE user_id = $1 AND campaign_id = $2",
		userID, campaignID,
	)
	return err
}

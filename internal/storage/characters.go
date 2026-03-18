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
	CharacterName string
	UpdatedAt     time.Time
}

func (s *Store) SetCharacterMapping(ctx context.Context, m CharacterMapping) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO character_mappings (user_id, guild_id, character_name, updated_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, guild_id) DO UPDATE SET character_name = $3, updated_at = $4`,
		m.UserID, m.GuildID, m.CharacterName, time.Now().UTC(),
	)
	return err
}

func (s *Store) GetCharacterMappings(ctx context.Context, guildID string) ([]CharacterMapping, error) {
	rows, err := s.Pool.Query(ctx,
		"SELECT user_id, guild_id, character_name, updated_at FROM character_mappings WHERE guild_id = $1",
		guildID,
	)
	if err != nil {
		return nil, fmt.Errorf("query character mappings: %w", err)
	}
	defer rows.Close()

	var mappings []CharacterMapping
	for rows.Next() {
		var m CharacterMapping
		if err := rows.Scan(&m.UserID, &m.GuildID, &m.CharacterName, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan mapping: %w", err)
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

func (s *Store) GetCharacterName(ctx context.Context, userID, guildID string) (string, error) {
	var name string
	err := s.Pool.QueryRow(ctx,
		"SELECT character_name FROM character_mappings WHERE user_id = $1 AND guild_id = $2",
		userID, guildID,
	).Scan(&name)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return name, err
}

func (s *Store) DeleteCharacterMapping(ctx context.Context, userID, guildID string) error {
	_, err := s.Pool.Exec(ctx,
		"DELETE FROM character_mappings WHERE user_id = $1 AND guild_id = $2",
		userID, guildID,
	)
	return err
}

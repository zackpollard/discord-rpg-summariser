package storage

import (
	"context"
	"fmt"
	"time"
)

type DiscordUser struct {
	UserID      string
	GuildID     string
	Username    string
	DisplayName string
	UpdatedAt   time.Time
}

func (s *Store) UpsertDiscordUser(ctx context.Context, u DiscordUser) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO discord_users (user_id, guild_id, username, display_name, updated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (user_id, guild_id) DO UPDATE SET username = $3, display_name = $4, updated_at = NOW()`,
		u.UserID, u.GuildID, u.Username, u.DisplayName,
	)
	return err
}

func (s *Store) UpsertDiscordUsers(ctx context.Context, users []DiscordUser) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, u := range users {
		_, err := tx.Exec(ctx,
			`INSERT INTO discord_users (user_id, guild_id, username, display_name, updated_at)
			 VALUES ($1, $2, $3, $4, NOW())
			 ON CONFLICT (user_id, guild_id) DO UPDATE SET username = $3, display_name = $4, updated_at = NOW()`,
			u.UserID, u.GuildID, u.Username, u.DisplayName,
		)
		if err != nil {
			return fmt.Errorf("upsert user %s: %w", u.UserID, err)
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) GetDiscordUsers(ctx context.Context, guildID string) ([]DiscordUser, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT user_id, guild_id, username, display_name, updated_at
		 FROM discord_users WHERE guild_id = $1 ORDER BY display_name`, guildID,
	)
	if err != nil {
		return nil, fmt.Errorf("query discord users: %w", err)
	}
	defer rows.Close()

	var users []DiscordUser
	for rows.Next() {
		var u DiscordUser
		if err := rows.Scan(&u.UserID, &u.GuildID, &u.Username, &u.DisplayName, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) GetDiscordUser(ctx context.Context, userID, guildID string) (*DiscordUser, error) {
	var u DiscordUser
	err := s.Pool.QueryRow(ctx,
		`SELECT user_id, guild_id, username, display_name, updated_at
		 FROM discord_users WHERE user_id = $1 AND guild_id = $2`, userID, guildID,
	).Scan(&u.UserID, &u.GuildID, &u.Username, &u.DisplayName, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

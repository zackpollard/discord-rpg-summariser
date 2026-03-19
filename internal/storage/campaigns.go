package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Campaign struct {
	ID                 int64
	GuildID            string
	Name               string
	Description        string
	IsActive           bool
	DMUserID           *string
	TelegramDMUserID   *int64
	Recap              string
	RecapGeneratedAt   *time.Time
	CreatedAt          time.Time
}

func (s *Store) CreateCampaign(ctx context.Context, guildID, name, description string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO campaigns (guild_id, name, description) VALUES ($1, $2, $3) RETURNING id`,
		guildID, name, description,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create campaign: %w", err)
	}
	return id, nil
}

func (s *Store) GetCampaign(ctx context.Context, id int64) (*Campaign, error) {
	var c Campaign
	err := s.Pool.QueryRow(ctx,
		`SELECT id, guild_id, name, description, is_active, dm_user_id, telegram_dm_user_id, recap, recap_generated_at, created_at FROM campaigns WHERE id = $1`, id,
	).Scan(&c.ID, &c.GuildID, &c.Name, &c.Description, &c.IsActive, &c.DMUserID, &c.TelegramDMUserID, &c.Recap, &c.RecapGeneratedAt, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) ListCampaigns(ctx context.Context, guildID string) ([]Campaign, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, guild_id, name, description, is_active, dm_user_id, telegram_dm_user_id, recap, recap_generated_at, created_at
		 FROM campaigns WHERE guild_id = $1 ORDER BY created_at`, guildID,
	)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []Campaign
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.ID, &c.GuildID, &c.Name, &c.Description, &c.IsActive, &c.DMUserID, &c.TelegramDMUserID, &c.Recap, &c.RecapGeneratedAt, &c.CreatedAt); err != nil {
			return nil, err
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, rows.Err()
}

// SetActiveCampaign sets the given campaign as active, deactivating any other
// active campaign for the same guild in a single transaction.
func (s *Store) SetActiveCampaign(ctx context.Context, guildID string, campaignID int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `UPDATE campaigns SET is_active = false WHERE guild_id = $1 AND is_active = true`, guildID)
	if err != nil {
		return fmt.Errorf("deactivate old: %w", err)
	}

	_, err = tx.Exec(ctx, `UPDATE campaigns SET is_active = true WHERE id = $1 AND guild_id = $2`, campaignID, guildID)
	if err != nil {
		return fmt.Errorf("activate new: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Store) GetActiveCampaign(ctx context.Context, guildID string) (*Campaign, error) {
	var c Campaign
	err := s.Pool.QueryRow(ctx,
		`SELECT id, guild_id, name, description, is_active, dm_user_id, telegram_dm_user_id, recap, recap_generated_at, created_at
		 FROM campaigns WHERE guild_id = $1 AND is_active = true`, guildID,
	).Scan(&c.ID, &c.GuildID, &c.Name, &c.Description, &c.IsActive, &c.DMUserID, &c.TelegramDMUserID, &c.Recap, &c.RecapGeneratedAt, &c.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) UpdateCampaignRecap(ctx context.Context, campaignID int64, recap string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE campaigns SET recap = $1, recap_generated_at = NOW() WHERE id = $2`, recap, campaignID)
	return err
}

func (s *Store) SetCampaignDM(ctx context.Context, campaignID int64, dmUserID string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE campaigns SET dm_user_id = $1 WHERE id = $2`, dmUserID, campaignID)
	return err
}

// GetOrCreateActiveCampaign returns the active campaign for a guild, creating
// a "Default Campaign" if none exists.
func (s *Store) GetOrCreateActiveCampaign(ctx context.Context, guildID string) (*Campaign, error) {
	c, err := s.GetActiveCampaign(ctx, guildID)
	if err != nil {
		return nil, err
	}
	if c != nil {
		return c, nil
	}

	id, err := s.CreateCampaign(ctx, guildID, "Default Campaign", "")
	if err != nil {
		return nil, fmt.Errorf("auto-create campaign: %w", err)
	}
	if err := s.SetActiveCampaign(ctx, guildID, id); err != nil {
		return nil, fmt.Errorf("set auto-created campaign active: %w", err)
	}
	return s.GetCampaign(ctx, id)
}

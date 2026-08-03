package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Campaign struct {
	ID                      int64
	GuildID                 string
	Name                    string
	Description             string
	GameSystem              string
	IsActive                bool
	DMUserID                *string
	TelegramDMUserID        *int64
	Recap                   string
	RecapGeneratedAt        *time.Time
	PreviouslyOn            string
	PreviouslyOnGeneratedAt *time.Time
	CreatedAt               time.Time
}

const campaignCols = `id, guild_id, name, description, game_system, is_active, dm_user_id, telegram_dm_user_id, recap, recap_generated_at, previously_on, previously_on_generated_at, created_at`

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
		`SELECT `+campaignCols+` FROM campaigns WHERE id = $1`, id,
	).Scan(&c.ID, &c.GuildID, &c.Name, &c.Description, &c.GameSystem, &c.IsActive, &c.DMUserID, &c.TelegramDMUserID, &c.Recap, &c.RecapGeneratedAt, &c.PreviouslyOn, &c.PreviouslyOnGeneratedAt, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) ListCampaigns(ctx context.Context, guildID string) ([]Campaign, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+campaignCols+` FROM campaigns WHERE guild_id = $1 ORDER BY created_at`, guildID,
	)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []Campaign
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.ID, &c.GuildID, &c.Name, &c.Description, &c.GameSystem, &c.IsActive, &c.DMUserID, &c.TelegramDMUserID, &c.Recap, &c.RecapGeneratedAt, &c.PreviouslyOn, &c.PreviouslyOnGeneratedAt, &c.CreatedAt); err != nil {
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

	// Serialise per guild so concurrent activations can't both observe the
	// other's pre-deactivation state and collide on idx_campaigns_guild_active.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, guildID); err != nil {
		return fmt.Errorf("lock guild campaigns: %w", err)
	}

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
		`SELECT `+campaignCols+` FROM campaigns WHERE guild_id = $1 AND is_active = true`, guildID,
	).Scan(&c.ID, &c.GuildID, &c.Name, &c.Description, &c.GameSystem, &c.IsActive, &c.DMUserID, &c.TelegramDMUserID, &c.Recap, &c.RecapGeneratedAt, &c.PreviouslyOn, &c.PreviouslyOnGeneratedAt, &c.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateCampaign updates the editable campaign fields.
func (s *Store) UpdateCampaign(ctx context.Context, id int64, name, description, gameSystem string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE campaigns SET name = $1, description = $2, game_system = $3 WHERE id = $4`,
		name, description, gameSystem, id,
	)
	if err != nil {
		return fmt.Errorf("update campaign: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateCampaignRecap(ctx context.Context, campaignID int64, recap string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE campaigns SET recap = $1, recap_generated_at = NOW() WHERE id = $2`, recap, campaignID)
	return err
}

func (s *Store) UpdateCampaignPreviouslyOn(ctx context.Context, campaignID int64, text string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE campaigns SET previously_on = $1, previously_on_generated_at = NOW() WHERE id = $2`, text, campaignID)
	return err
}

func (s *Store) SetCampaignDM(ctx context.Context, campaignID int64, dmUserID string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE campaigns SET dm_user_id = $1 WHERE id = $2`, dmUserID, campaignID)
	return err
}

// GetOrCreateActiveCampaign returns the active campaign for a guild, creating
// a "Default Campaign" if none exists. The lookup and the creation run in a
// single transaction holding a per-guild advisory lock, so concurrent callers
// can't each create their own default campaign.
func (s *Store) GetOrCreateActiveCampaign(ctx context.Context, guildID string) (*Campaign, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, guildID); err != nil {
		return nil, fmt.Errorf("lock guild campaigns: %w", err)
	}

	var c Campaign
	err = tx.QueryRow(ctx,
		`SELECT `+campaignCols+` FROM campaigns WHERE guild_id = $1 AND is_active = true`, guildID,
	).Scan(&c.ID, &c.GuildID, &c.Name, &c.Description, &c.GameSystem, &c.IsActive, &c.DMUserID, &c.TelegramDMUserID, &c.Recap, &c.RecapGeneratedAt, &c.PreviouslyOn, &c.PreviouslyOnGeneratedAt, &c.CreatedAt)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx,
			`INSERT INTO campaigns (guild_id, name, description, is_active) VALUES ($1, 'Default Campaign', '', true)
			 RETURNING `+campaignCols, guildID,
		).Scan(&c.ID, &c.GuildID, &c.Name, &c.Description, &c.GameSystem, &c.IsActive, &c.DMUserID, &c.TelegramDMUserID, &c.Recap, &c.RecapGeneratedAt, &c.PreviouslyOn, &c.PreviouslyOnGeneratedAt, &c.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("auto-create campaign: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &c, nil
}

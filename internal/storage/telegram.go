package storage

import (
	"context"
	"fmt"
	"time"
)

// TelegramMessage represents a message captured from Telegram during a session.
type TelegramMessage struct {
	ID            int64
	SessionID     int64
	TelegramMsgID int64
	FromUserID    int64
	FromUsername  string
	FromDisplay   string
	Text          string
	SentAt        time.Time
	IsDM          bool
	CreatedAt     time.Time
}

// InsertTelegramMessages bulk-inserts Telegram messages for a session.
func (s *Store) InsertTelegramMessages(ctx context.Context, msgs []TelegramMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, m := range msgs {
		_, err := tx.Exec(ctx,
			`INSERT INTO telegram_messages (session_id, telegram_msg_id, from_user_id, from_username, from_display, text, sent_at, is_dm)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 ON CONFLICT (session_id, telegram_msg_id) DO NOTHING`,
			m.SessionID, m.TelegramMsgID, m.FromUserID, m.FromUsername, m.FromDisplay, m.Text, m.SentAt, m.IsDM,
		)
		if err != nil {
			return fmt.Errorf("insert telegram message: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetTelegramMessages returns Telegram messages for a session.
// If dmOnly is true, only messages from the DM are returned.
func (s *Store) GetTelegramMessages(ctx context.Context, sessionID int64, dmOnly bool) ([]TelegramMessage, error) {
	query := `SELECT id, session_id, telegram_msg_id, from_user_id, from_username, from_display, text, sent_at, is_dm, created_at
		FROM telegram_messages WHERE session_id = $1`
	if dmOnly {
		query += ` AND is_dm = true`
	}
	query += ` ORDER BY sent_at`

	rows, err := s.Pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query telegram messages: %w", err)
	}
	defer rows.Close()

	var msgs []TelegramMessage
	for rows.Next() {
		var m TelegramMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.TelegramMsgID, &m.FromUserID,
			&m.FromUsername, &m.FromDisplay, &m.Text, &m.SentAt, &m.IsDM, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan telegram message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// SetCampaignTelegramDM sets the Telegram user ID of the DM for a campaign.
func (s *Store) SetCampaignTelegramDM(ctx context.Context, campaignID int64, telegramUserID int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE campaigns SET telegram_dm_user_id = $1 WHERE id = $2`, telegramUserID, campaignID)
	return err
}

// DeleteTelegramMessages removes all Telegram messages for a session.
func (s *Store) DeleteTelegramMessages(ctx context.Context, sessionID int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM telegram_messages WHERE session_id = $1`, sessionID)
	return err
}

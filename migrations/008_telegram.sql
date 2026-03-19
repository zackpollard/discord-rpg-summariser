-- Add Telegram DM user ID to campaigns
ALTER TABLE campaigns ADD COLUMN telegram_dm_user_id BIGINT;

-- Store Telegram messages captured during sessions
CREATE TABLE telegram_messages (
    id              BIGSERIAL PRIMARY KEY,
    session_id      BIGINT NOT NULL REFERENCES sessions(id),
    telegram_msg_id BIGINT NOT NULL,
    from_user_id    BIGINT NOT NULL,
    from_username   TEXT NOT NULL DEFAULT '',
    from_display    TEXT NOT NULL DEFAULT '',
    text            TEXT NOT NULL,
    sent_at         TIMESTAMPTZ NOT NULL,
    is_dm           BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(session_id, telegram_msg_id)
);

CREATE INDEX idx_telegram_messages_session ON telegram_messages(session_id, sent_at);

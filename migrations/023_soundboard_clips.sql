CREATE TABLE soundboard_clips (
    id          BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    session_id  BIGINT REFERENCES sessions(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    audio_path  TEXT NOT NULL,
    start_time  DOUBLE PRECISION NOT NULL,
    end_time    DOUBLE PRECISION NOT NULL,
    user_ids    JSONB NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_soundboard_clips_campaign ON soundboard_clips(campaign_id);

CREATE TABLE sessions (
    id          BIGSERIAL PRIMARY KEY,
    guild_id    TEXT NOT NULL,
    channel_id  TEXT NOT NULL,
    started_at  TIMESTAMPTZ NOT NULL,
    ended_at    TIMESTAMPTZ,
    status      TEXT NOT NULL DEFAULT 'recording',
    audio_dir   TEXT NOT NULL,
    summary     TEXT,
    key_events  JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE transcript_segments (
    id             BIGSERIAL PRIMARY KEY,
    session_id     BIGINT NOT NULL REFERENCES sessions(id),
    user_id        TEXT NOT NULL,
    character_name TEXT,
    start_time     DOUBLE PRECISION NOT NULL,
    end_time       DOUBLE PRECISION NOT NULL,
    text           TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_segments_session ON transcript_segments(session_id, start_time);

CREATE TABLE character_mappings (
    user_id        TEXT NOT NULL,
    guild_id       TEXT NOT NULL,
    character_name TEXT NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, guild_id)
);

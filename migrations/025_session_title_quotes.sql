ALTER TABLE sessions ADD COLUMN title TEXT;
CREATE TABLE session_quotes (
    id         BIGSERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    speaker    TEXT NOT NULL,
    text       TEXT NOT NULL,
    start_time DOUBLE PRECISION NOT NULL,
    tone       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_session_quotes_session ON session_quotes(session_id);

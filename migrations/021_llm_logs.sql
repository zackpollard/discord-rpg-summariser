CREATE TABLE llm_logs (
    id          BIGSERIAL PRIMARY KEY,
    session_id  BIGINT REFERENCES sessions(id) ON DELETE CASCADE,
    operation   TEXT NOT NULL,
    prompt      TEXT NOT NULL,
    response    TEXT NOT NULL,
    error       TEXT,
    duration_ms INTEGER NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_llm_logs_session ON llm_logs(session_id);

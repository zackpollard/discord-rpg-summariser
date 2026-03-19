-- Voice enrollment: speaker embeddings extracted from session recordings,
-- used to identify speakers on shared microphones.
CREATE TABLE speaker_enrollments (
    id          BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id),
    user_id     TEXT NOT NULL,
    embedding   REAL[] NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, user_id)
);

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE embeddings (
    id          BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id),
    doc_type    TEXT NOT NULL,
    doc_id      BIGINT NOT NULL,
    session_id  BIGINT REFERENCES sessions(id) ON DELETE CASCADE,
    title       TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL,
    embedding   vector(768) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(campaign_id, doc_type, doc_id)
);

CREATE INDEX idx_embeddings_campaign ON embeddings(campaign_id);
CREATE INDEX idx_embeddings_vector ON embeddings USING hnsw (embedding vector_cosine_ops);

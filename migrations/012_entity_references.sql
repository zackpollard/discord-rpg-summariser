-- Cross-session entity references: track which sessions each entity appears in
-- and link entity detail pages to exact transcript moments.
CREATE TABLE entity_references (
    id          BIGSERIAL PRIMARY KEY,
    entity_id   BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    session_id  BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    segment_id  BIGINT REFERENCES transcript_segments(id) ON DELETE SET NULL,
    context     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (entity_id, segment_id)
);
CREATE INDEX idx_entity_refs_entity ON entity_references(entity_id);
CREATE INDEX idx_entity_refs_session ON entity_references(session_id);

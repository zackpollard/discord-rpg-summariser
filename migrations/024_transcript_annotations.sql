CREATE TABLE transcript_annotations (
    id              BIGSERIAL PRIMARY KEY,
    segment_id      BIGINT NOT NULL REFERENCES transcript_segments(id) ON DELETE CASCADE,
    session_id      BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    classification  TEXT NOT NULL DEFAULT 'narrative',
    corrected_text  TEXT,
    scene           TEXT,
    npc_voice       TEXT,
    merge_with_next BOOLEAN NOT NULL DEFAULT FALSE,
    tone            TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_annotations_segment ON transcript_annotations(segment_id);
CREATE INDEX idx_annotations_session ON transcript_annotations(session_id);

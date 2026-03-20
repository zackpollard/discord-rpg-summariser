CREATE TABLE entity_merges (
    id           BIGSERIAL PRIMARY KEY,
    campaign_id  BIGINT NOT NULL REFERENCES campaigns(id),
    kept_id      BIGINT NOT NULL REFERENCES entities(id),
    merged_id    BIGINT NOT NULL,
    merged_name  TEXT NOT NULL,
    merged_type  TEXT NOT NULL,
    merged_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_entity_merges_campaign ON entity_merges(campaign_id);

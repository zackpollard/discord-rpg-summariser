-- Campaigns
CREATE TABLE campaigns (
    id          BIGSERIAL PRIMARY KEY,
    guild_id    TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_campaigns_guild_active ON campaigns (guild_id) WHERE is_active = true;

-- Auto-create default campaign for existing data
INSERT INTO campaigns (guild_id, name, is_active)
SELECT DISTINCT guild_id, 'Default Campaign', true FROM sessions;

-- Link sessions to campaigns
ALTER TABLE sessions ADD COLUMN campaign_id BIGINT REFERENCES campaigns(id);
UPDATE sessions s SET campaign_id = c.id
FROM campaigns c WHERE c.guild_id = s.guild_id AND c.is_active = true;
ALTER TABLE sessions ALTER COLUMN campaign_id SET NOT NULL;

-- Re-key character_mappings from guild to campaign scope
ALTER TABLE character_mappings ADD COLUMN campaign_id BIGINT REFERENCES campaigns(id);
UPDATE character_mappings cm SET campaign_id = c.id
FROM campaigns c WHERE c.guild_id = cm.guild_id AND c.is_active = true;
ALTER TABLE character_mappings ALTER COLUMN campaign_id SET NOT NULL;
ALTER TABLE character_mappings DROP CONSTRAINT character_mappings_pkey;
ALTER TABLE character_mappings ADD PRIMARY KEY (user_id, campaign_id);

-- Knowledge base: entities
CREATE TABLE entities (
    id          BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id),
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, name, type)
);
CREATE INDEX idx_entities_campaign_type ON entities (campaign_id, type);

-- Entity notes: accumulated per-session observations
CREATE TABLE entity_notes (
    id         BIGSERIAL PRIMARY KEY,
    entity_id  BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    session_id BIGINT NOT NULL REFERENCES sessions(id),
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_entity_notes_entity ON entity_notes (entity_id);

-- Entity relationships
CREATE TABLE entity_relationships (
    id           BIGSERIAL PRIMARY KEY,
    campaign_id  BIGINT NOT NULL REFERENCES campaigns(id),
    source_id    BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    target_id    BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    relationship TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    session_id   BIGINT REFERENCES sessions(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_id, target_id, relationship)
);
CREATE INDEX idx_entity_rels_source ON entity_relationships (source_id);
CREATE INDEX idx_entity_rels_target ON entity_relationships (target_id);

-- Creature-specific metadata (1:1 with entities of type "creature")
CREATE TABLE creature_stats (
    id               BIGSERIAL PRIMARY KEY,
    entity_id        BIGINT NOT NULL UNIQUE REFERENCES entities(id) ON DELETE CASCADE,
    creature_type    TEXT NOT NULL DEFAULT '',
    challenge_rating TEXT NOT NULL DEFAULT '',
    armor_class      INTEGER,
    hit_points       TEXT NOT NULL DEFAULT '',
    abilities        TEXT NOT NULL DEFAULT '',
    loot             TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_creature_stats_entity ON creature_stats(entity_id);

ALTER TABLE combat_actions ADD COLUMN IF NOT EXISTS actor_entity_id BIGINT REFERENCES entities(id) ON DELETE SET NULL;
ALTER TABLE combat_actions ADD COLUMN IF NOT EXISTS target_entity_id BIGINT REFERENCES entities(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_combat_actions_actor_entity ON combat_actions(actor_entity_id);
CREATE INDEX IF NOT EXISTS idx_combat_actions_target_entity ON combat_actions(target_entity_id);

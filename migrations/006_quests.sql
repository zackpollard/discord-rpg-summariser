CREATE TABLE quests (
    id          BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active',
    giver       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, name)
);
CREATE INDEX idx_quests_campaign_status ON quests (campaign_id, status);

CREATE TABLE quest_updates (
    id         BIGSERIAL PRIMARY KEY,
    quest_id   BIGINT NOT NULL REFERENCES quests(id) ON DELETE CASCADE,
    session_id BIGINT NOT NULL REFERENCES sessions(id),
    content    TEXT NOT NULL,
    new_status TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_quest_updates_quest ON quest_updates (quest_id);

CREATE TABLE combat_encounters (
    id          BIGSERIAL PRIMARY KEY,
    session_id  BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id),
    name        TEXT NOT NULL DEFAULT '',
    start_time  DOUBLE PRECISION NOT NULL,
    end_time    DOUBLE PRECISION NOT NULL,
    summary     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_combat_encounters_session ON combat_encounters(session_id);

CREATE TABLE combat_actions (
    id           BIGSERIAL PRIMARY KEY,
    encounter_id BIGINT NOT NULL REFERENCES combat_encounters(id) ON DELETE CASCADE,
    actor        TEXT NOT NULL,
    action_type  TEXT NOT NULL,
    target       TEXT,
    detail       TEXT NOT NULL DEFAULT '',
    damage       INTEGER,
    round        INTEGER,
    timestamp    DOUBLE PRECISION,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_combat_actions_encounter ON combat_actions(encounter_id);

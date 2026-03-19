-- Shared microphone configuration: maps one Discord user to two speakers
CREATE TABLE shared_mics (
    id                BIGSERIAL PRIMARY KEY,
    campaign_id       BIGINT NOT NULL REFERENCES campaigns(id),
    discord_user_id   TEXT NOT NULL,
    speaker_a_user_id TEXT NOT NULL,  -- typically the DM
    speaker_b_user_id TEXT NOT NULL,  -- the partner (synthetic ID)
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, discord_user_id)
);

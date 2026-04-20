-- Cache generated TTS audio metadata (files stored on disk).
CREATE TABLE tts_audio_cache (
    id            BIGSERIAL PRIMARY KEY,
    campaign_id   BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    source        TEXT NOT NULL,       -- 'recap' or 'previously-on'
    voice_key     TEXT NOT NULL,       -- 'user:{discordID}' or 'profile:{profileID}'
    audio_path    TEXT NOT NULL,
    generated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, source, voice_key)
);

-- Persist "previously on" text so it's not regenerated via LLM each time.
ALTER TABLE campaigns ADD COLUMN previously_on TEXT NOT NULL DEFAULT '';
ALTER TABLE campaigns ADD COLUMN previously_on_generated_at TIMESTAMPTZ;

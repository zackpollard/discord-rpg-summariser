-- Add game_system column to campaigns, used for transcription prompt biasing
-- and display context. Defaults to 'Dungeons & Dragons' for existing campaigns.
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS game_system TEXT NOT NULL DEFAULT 'Dungeons & Dragons';

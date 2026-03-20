-- Full-text search on transcript segments
ALTER TABLE transcript_segments ADD COLUMN IF NOT EXISTS tsv tsvector;

-- Populate existing rows
UPDATE transcript_segments SET tsv = to_tsvector('english', text) WHERE tsv IS NULL;

-- GIN index for fast full-text search
CREATE INDEX IF NOT EXISTS idx_transcript_segments_tsv ON transcript_segments USING GIN(tsv);

-- Trigger to auto-update tsv on insert/update
CREATE OR REPLACE FUNCTION transcript_segments_tsv_trigger() RETURNS trigger AS $$
BEGIN
    NEW.tsv := to_tsvector('english', NEW.text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_transcript_segments_tsv ON transcript_segments;
CREATE TRIGGER trg_transcript_segments_tsv
    BEFORE INSERT OR UPDATE OF text ON transcript_segments
    FOR EACH ROW EXECUTE FUNCTION transcript_segments_tsv_trigger();

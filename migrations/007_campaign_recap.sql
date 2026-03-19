ALTER TABLE campaigns ADD COLUMN recap TEXT DEFAULT '';
ALTER TABLE campaigns ADD COLUMN recap_generated_at TIMESTAMPTZ;

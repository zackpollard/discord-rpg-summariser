-- Character names are now resolved at display time from character_mappings.
-- Remove the denormalized column from transcript_segments.
ALTER TABLE transcript_segments DROP COLUMN IF EXISTS character_name;

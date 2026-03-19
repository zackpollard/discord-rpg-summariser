-- Character names are now resolved at display time from character_mappings.
-- Clear any previously denormalized values from transcript segments.
UPDATE transcript_segments SET character_name = NULL WHERE character_name IS NOT NULL;

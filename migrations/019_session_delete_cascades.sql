-- Add ON DELETE CASCADE to FK constraints that reference sessions(id),
-- enabling clean session deletion without manual child-row cleanup.

ALTER TABLE transcript_segments
    DROP CONSTRAINT IF EXISTS transcript_segments_session_id_fkey,
    ADD CONSTRAINT transcript_segments_session_id_fkey
        FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE;

ALTER TABLE telegram_messages
    DROP CONSTRAINT IF EXISTS telegram_messages_session_id_fkey,
    ADD CONSTRAINT telegram_messages_session_id_fkey
        FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE;

ALTER TABLE entity_notes
    DROP CONSTRAINT IF EXISTS entity_notes_session_id_fkey,
    ADD CONSTRAINT entity_notes_session_id_fkey
        FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE;

ALTER TABLE entity_relationships
    DROP CONSTRAINT IF EXISTS entity_relationships_session_id_fkey,
    ADD CONSTRAINT entity_relationships_session_id_fkey
        FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE SET NULL;

ALTER TABLE quest_updates
    DROP CONSTRAINT IF EXISTS quest_updates_session_id_fkey,
    ADD CONSTRAINT quest_updates_session_id_fkey
        FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE;

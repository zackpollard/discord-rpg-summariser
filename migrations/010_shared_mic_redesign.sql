-- Redesign shared_mics: remove DM-centric speaker_a/speaker_b columns,
-- replace with a single partner_user_id. The mic owner (discord_user_id)
-- is one speaker; partner_user_id is the other (real Discord ID or synthetic).
ALTER TABLE shared_mics DROP COLUMN speaker_a_user_id;
ALTER TABLE shared_mics DROP COLUMN speaker_b_user_id;
ALTER TABLE shared_mics ADD COLUMN partner_user_id TEXT NOT NULL DEFAULT '';

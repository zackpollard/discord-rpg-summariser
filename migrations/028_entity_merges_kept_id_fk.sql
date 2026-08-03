-- entity_merges is an audit log, and the kept entity of a merge can itself be
-- merged away later. The restricting FK on kept_id made that impossible: the
-- DELETE of the merged entity aborted the whole merge transaction. Drop the
-- constraint and treat kept_id as a plain audit value, matching merged_id
-- (which already has no FK), so the history survives entity deletion.
ALTER TABLE entity_merges
    DROP CONSTRAINT IF EXISTS entity_merges_kept_id_fkey;

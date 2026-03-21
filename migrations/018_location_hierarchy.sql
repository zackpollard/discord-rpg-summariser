ALTER TABLE entities ADD COLUMN parent_entity_id BIGINT REFERENCES entities(id) ON DELETE SET NULL;
CREATE INDEX idx_entities_parent ON entities(parent_entity_id);

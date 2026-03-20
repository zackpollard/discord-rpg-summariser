package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Entity struct {
	ID          int64
	CampaignID  int64
	Name        string
	Type        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type EntityNote struct {
	ID        int64
	EntityID  int64
	SessionID int64
	Content   string
	CreatedAt time.Time
}

type EntityRelationship struct {
	ID           int64
	CampaignID   int64
	SourceID     int64
	TargetID     int64
	Relationship string
	Description  string
	SessionID    *int64
	CreatedAt    time.Time
}

func (s *Store) UpsertEntity(ctx context.Context, campaignID int64, name, typ, description string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO entities (campaign_id, name, type, description)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (campaign_id, name, type) DO UPDATE SET
		   description = CASE WHEN $4 = '' THEN entities.description ELSE $4 END,
		   updated_at = NOW()
		 RETURNING id`,
		campaignID, name, typ, description,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert entity: %w", err)
	}
	return id, nil
}

func (s *Store) GetEntity(ctx context.Context, id int64) (*Entity, error) {
	var e Entity
	err := s.Pool.QueryRow(ctx,
		`SELECT id, campaign_id, name, type, description, created_at, updated_at
		 FROM entities WHERE id = $1`, id,
	).Scan(&e.ID, &e.CampaignID, &e.Name, &e.Type, &e.Description, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *Store) ListEntities(ctx context.Context, campaignID int64, typeFilter, search string, limit, offset int) ([]Entity, error) {
	query := `SELECT id, campaign_id, name, type, description, created_at, updated_at
		 FROM entities WHERE campaign_id = $1`
	args := []any{campaignID}
	argN := 2

	if typeFilter != "" {
		query += fmt.Sprintf(` AND type = $%d`, argN)
		args = append(args, typeFilter)
		argN++
	}
	if search != "" {
		query += fmt.Sprintf(` AND name ILIKE $%d`, argN)
		args = append(args, "%"+search+"%")
		argN++
	}

	query += ` ORDER BY name`
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argN, argN+1)
	args = append(args, limit, offset)

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.CampaignID, &e.Name, &e.Type, &e.Description, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}

func (s *Store) AddEntityNote(ctx context.Context, entityID, sessionID int64, content string) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO entity_notes (entity_id, session_id, content) VALUES ($1, $2, $3)`,
		entityID, sessionID, content,
	)
	return err
}

func (s *Store) GetEntityNotes(ctx context.Context, entityID int64) ([]EntityNote, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, entity_id, session_id, content, created_at
		 FROM entity_notes WHERE entity_id = $1 ORDER BY created_at`, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("get entity notes: %w", err)
	}
	defer rows.Close()

	var notes []EntityNote
	for rows.Next() {
		var n EntityNote
		if err := rows.Scan(&n.ID, &n.EntityID, &n.SessionID, &n.Content, &n.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

func (s *Store) UpsertEntityRelationship(ctx context.Context, campaignID, sourceID, targetID int64, relationship, description string, sessionID *int64) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO entity_relationships (campaign_id, source_id, target_id, relationship, description, session_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (source_id, target_id, relationship) DO UPDATE SET
		   description = $5, session_id = $6`,
		campaignID, sourceID, targetID, relationship, description, sessionID,
	)
	return err
}

// GetEntityRelationships returns all relationships where the entity is source OR target.
func (s *Store) GetEntityRelationships(ctx context.Context, entityID int64) ([]EntityRelationship, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, campaign_id, source_id, target_id, relationship, description, session_id, created_at
		 FROM entity_relationships WHERE source_id = $1 OR target_id = $1 ORDER BY created_at`, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("get entity relationships: %w", err)
	}
	defer rows.Close()

	var rels []EntityRelationship
	for rows.Next() {
		var r EntityRelationship
		if err := rows.Scan(&r.ID, &r.CampaignID, &r.SourceID, &r.TargetID, &r.Relationship, &r.Description, &r.SessionID, &r.CreatedAt); err != nil {
			return nil, err
		}
		rels = append(rels, r)
	}
	return rels, rows.Err()
}

// EnsurePCEntities upserts a "pc" entity for each character name and returns
// a map from character name to entity ID.
func (s *Store) EnsurePCEntities(ctx context.Context, campaignID int64, characterNames []string) (map[string]int64, error) {
	result := make(map[string]int64, len(characterNames))
	for _, name := range characterNames {
		id, err := s.UpsertEntity(ctx, campaignID, name, "pc", "")
		if err != nil {
			return nil, fmt.Errorf("ensure PC entity %q: %w", name, err)
		}
		result[name] = id
	}
	return result, nil
}

// GetEntityByName finds an entity by campaign, name and type.
func (s *Store) GetEntityByName(ctx context.Context, campaignID int64, name, typ string) (*Entity, error) {
	var e Entity
	err := s.Pool.QueryRow(ctx,
		`SELECT id, campaign_id, name, type, description, created_at, updated_at
		 FROM entities WHERE campaign_id = $1 AND name = $2 AND type = $3`, campaignID, name, typ,
	).Scan(&e.ID, &e.CampaignID, &e.Name, &e.Type, &e.Description, &e.CreatedAt, &e.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// GetCampaignRelationshipGraph returns all entities and all relationships for a campaign,
// suitable for rendering a relationship graph.
func (s *Store) GetCampaignRelationshipGraph(ctx context.Context, campaignID int64) ([]Entity, []EntityRelationship, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, campaign_id, name, type, description, created_at, updated_at
		 FROM entities WHERE campaign_id = $1 ORDER BY name`, campaignID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get campaign entities for graph: %w", err)
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.CampaignID, &e.Name, &e.Type, &e.Description, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, nil, err
		}
		entities = append(entities, e)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	relRows, err := s.Pool.Query(ctx,
		`SELECT id, campaign_id, source_id, target_id, relationship, description, session_id, created_at
		 FROM entity_relationships WHERE campaign_id = $1 ORDER BY created_at`, campaignID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get campaign relationships for graph: %w", err)
	}
	defer relRows.Close()

	var rels []EntityRelationship
	for relRows.Next() {
		var r EntityRelationship
		if err := relRows.Scan(&r.ID, &r.CampaignID, &r.SourceID, &r.TargetID, &r.Relationship, &r.Description, &r.SessionID, &r.CreatedAt); err != nil {
			return nil, nil, err
		}
		rels = append(rels, r)
	}
	if err := relRows.Err(); err != nil {
		return nil, nil, err
	}

	return entities, rels, nil
}

// MergeEntities merges the entity identified by mergeID into keepID within a
// single transaction. Notes, references, and relationships are moved to the
// kept entity, the merged entity's description is appended when it differs,
// an audit row is inserted into entity_merges, and the merged entity is deleted.
func (s *Store) MergeEntities(ctx context.Context, campaignID, keepID, mergeID int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin merge tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Fetch both entities to record audit info and merge descriptions.
	var keepDesc, mergeDesc, mergeName, mergeType string
	err = tx.QueryRow(ctx,
		`SELECT description FROM entities WHERE id = $1 AND campaign_id = $2`, keepID, campaignID,
	).Scan(&keepDesc)
	if err != nil {
		return fmt.Errorf("fetch kept entity: %w", err)
	}
	err = tx.QueryRow(ctx,
		`SELECT name, type, description FROM entities WHERE id = $1 AND campaign_id = $2`, mergeID, campaignID,
	).Scan(&mergeName, &mergeType, &mergeDesc)
	if err != nil {
		return fmt.Errorf("fetch merged entity: %w", err)
	}

	// 1. Move entity_notes from mergeID to keepID.
	if _, err := tx.Exec(ctx,
		`UPDATE entity_notes SET entity_id = $1 WHERE entity_id = $2`,
		keepID, mergeID,
	); err != nil {
		return fmt.Errorf("move entity notes: %w", err)
	}

	// 2. Move entity_references from mergeID to keepID (skip duplicates).
	if _, err := tx.Exec(ctx,
		`UPDATE entity_references SET entity_id = $1
		 WHERE entity_id = $2
		   AND id NOT IN (
		     SELECT er.id FROM entity_references er
		     JOIN entity_references existing ON existing.entity_id = $1 AND existing.segment_id = er.segment_id
		     WHERE er.entity_id = $2 AND er.segment_id IS NOT NULL
		   )`,
		keepID, mergeID,
	); err != nil {
		return fmt.Errorf("move entity references: %w", err)
	}
	// Delete any remaining references that couldn't be moved (duplicates).
	if _, err := tx.Exec(ctx,
		`DELETE FROM entity_references WHERE entity_id = $1`, mergeID,
	); err != nil {
		return fmt.Errorf("delete duplicate entity references: %w", err)
	}

	// 3. Move entity_relationships: update source_id and target_id.
	// Delete conflicting relationships first (same source, target, relationship combo),
	// then move the rest.

	// Remove relationships that would conflict when updating source_id.
	if _, err := tx.Exec(ctx,
		`DELETE FROM entity_relationships
		 WHERE source_id = $1
		   AND EXISTS (
		     SELECT 1 FROM entity_relationships r2
		     WHERE r2.source_id = $2
		       AND r2.target_id = entity_relationships.target_id
		       AND r2.relationship = entity_relationships.relationship
		   )`,
		mergeID, keepID,
	); err != nil {
		return fmt.Errorf("delete conflicting source relationships: %w", err)
	}
	// Move remaining source relationships.
	if _, err := tx.Exec(ctx,
		`UPDATE entity_relationships SET source_id = $1 WHERE source_id = $2`,
		keepID, mergeID,
	); err != nil {
		return fmt.Errorf("move source relationships: %w", err)
	}

	// Remove relationships that would conflict when updating target_id.
	if _, err := tx.Exec(ctx,
		`DELETE FROM entity_relationships
		 WHERE target_id = $1
		   AND EXISTS (
		     SELECT 1 FROM entity_relationships r2
		     WHERE r2.target_id = $2
		       AND r2.source_id = entity_relationships.source_id
		       AND r2.relationship = entity_relationships.relationship
		   )`,
		mergeID, keepID,
	); err != nil {
		return fmt.Errorf("delete conflicting target relationships: %w", err)
	}
	// Move remaining target relationships.
	if _, err := tx.Exec(ctx,
		`UPDATE entity_relationships SET target_id = $1 WHERE target_id = $2`,
		keepID, mergeID,
	); err != nil {
		return fmt.Errorf("move target relationships: %w", err)
	}

	// Remove any self-referential relationships that may have been created.
	if _, err := tx.Exec(ctx,
		`DELETE FROM entity_relationships WHERE source_id = $1 AND target_id = $1`,
		keepID,
	); err != nil {
		return fmt.Errorf("delete self-referential relationships: %w", err)
	}

	// 4. Append merged entity's description to kept entity's description if they differ.
	if mergeDesc != "" && mergeDesc != keepDesc {
		newDesc := keepDesc
		if newDesc != "" {
			newDesc += "\n\n"
		}
		newDesc += mergeDesc
		if _, err := tx.Exec(ctx,
			`UPDATE entities SET description = $1, updated_at = NOW() WHERE id = $2`,
			newDesc, keepID,
		); err != nil {
			return fmt.Errorf("update kept entity description: %w", err)
		}
	}

	// 5. Insert audit row into entity_merges.
	if _, err := tx.Exec(ctx,
		`INSERT INTO entity_merges (campaign_id, kept_id, merged_id, merged_name, merged_type)
		 VALUES ($1, $2, $3, $4, $5)`,
		campaignID, keepID, mergeID, mergeName, mergeType,
	); err != nil {
		return fmt.Errorf("insert entity merge audit: %w", err)
	}

	// 6. Delete the merged entity.
	if _, err := tx.Exec(ctx,
		`DELETE FROM entities WHERE id = $1`, mergeID,
	); err != nil {
		return fmt.Errorf("delete merged entity: %w", err)
	}

	return tx.Commit(ctx)
}

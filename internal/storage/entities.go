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

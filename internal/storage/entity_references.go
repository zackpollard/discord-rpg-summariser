package storage

import (
	"context"
	"fmt"
	"time"
)

// EntityReference represents a mention of an entity within a transcript segment.
type EntityReference struct {
	ID        int64
	EntityID  int64
	SessionID int64
	SegmentID *int64
	StartTime float64
	EndTime   float64
	Context   string
	CreatedAt time.Time
}

// SessionAppearance summarises an entity's presence in a single session.
type SessionAppearance struct {
	SessionID    int64
	StartedAt    time.Time
	MentionCount int
}

// InsertEntityReferences bulk-inserts entity references, skipping duplicates
// on the (entity_id, segment_id) unique constraint.
func (s *Store) InsertEntityReferences(ctx context.Context, refs []EntityReference) error {
	if len(refs) == 0 {
		return nil
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, ref := range refs {
		_, err := tx.Exec(ctx,
			`INSERT INTO entity_references (entity_id, session_id, segment_id, context)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (entity_id, segment_id) DO NOTHING`,
			ref.EntityID, ref.SessionID, ref.SegmentID, ref.Context,
		)
		if err != nil {
			return fmt.Errorf("insert entity reference: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetEntityReferences returns references for an entity with segment timing info.
func (s *Store) GetEntityReferences(ctx context.Context, entityID int64, limit, offset int) ([]EntityReference, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT er.id, er.entity_id, er.session_id, er.segment_id,
		        COALESCE(ts.start_time, 0), COALESCE(ts.end_time, 0),
		        er.context, er.created_at
		 FROM entity_references er
		 LEFT JOIN transcript_segments ts ON ts.id = er.segment_id
		 WHERE er.entity_id = $1
		 ORDER BY er.created_at
		 LIMIT $2 OFFSET $3`, entityID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get entity references: %w", err)
	}
	defer rows.Close()

	var refs []EntityReference
	for rows.Next() {
		var r EntityReference
		if err := rows.Scan(&r.ID, &r.EntityID, &r.SessionID, &r.SegmentID,
			&r.StartTime, &r.EndTime, &r.Context, &r.CreatedAt); err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

// GetEntitySessionAppearances returns distinct sessions where the entity is
// mentioned, along with the count of mentions per session.
func (s *Store) GetEntitySessionAppearances(ctx context.Context, entityID int64) ([]SessionAppearance, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT er.session_id, s.started_at, COUNT(*) AS mention_count
		 FROM entity_references er
		 JOIN sessions s ON s.id = er.session_id
		 WHERE er.entity_id = $1
		 GROUP BY er.session_id, s.started_at
		 ORDER BY s.started_at`, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("get entity session appearances: %w", err)
	}
	defer rows.Close()

	var appearances []SessionAppearance
	for rows.Next() {
		var a SessionAppearance
		if err := rows.Scan(&a.SessionID, &a.StartedAt, &a.MentionCount); err != nil {
			return nil, err
		}
		appearances = append(appearances, a)
	}
	return appearances, rows.Err()
}

// DeleteEntityReferencesForSession removes all entity references for a session,
// used when reprocessing.
func (s *Store) DeleteEntityReferencesForSession(ctx context.Context, sessionID int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM entity_references WHERE session_id = $1`, sessionID)
	return err
}

// EntityTimelineEntry summarises an entity's activity across sessions for
// timeline visualization.
type EntityTimelineEntry struct {
	EntityID      int64     `json:"entity_id"`
	EntityName    string    `json:"entity_name"`
	EntityType    string    `json:"entity_type"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	SessionCount  int       `json:"session_count"`
	TotalMentions int       `json:"total_mentions"`
}

// GetEntityTimeline returns a timeline summary for every entity in a campaign
// that has at least one entity_reference, ordered by first_seen.
func (s *Store) GetEntityTimeline(ctx context.Context, campaignID int64) ([]EntityTimelineEntry, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT e.id, e.name, e.type,
		        MIN(s.started_at) AS first_seen,
		        MAX(s.started_at) AS last_seen,
		        COUNT(DISTINCT er.session_id) AS session_count,
		        COUNT(*) AS total_mentions
		 FROM entity_references er
		 JOIN entities e ON e.id = er.entity_id
		 JOIN sessions s ON s.id = er.session_id
		 WHERE e.campaign_id = $1
		 GROUP BY e.id, e.name, e.type
		 ORDER BY first_seen, e.name`, campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("get entity timeline: %w", err)
	}
	defer rows.Close()

	var entries []EntityTimelineEntry
	for rows.Next() {
		var entry EntityTimelineEntry
		if err := rows.Scan(&entry.EntityID, &entry.EntityName, &entry.EntityType,
			&entry.FirstSeen, &entry.LastSeen, &entry.SessionCount, &entry.TotalMentions); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

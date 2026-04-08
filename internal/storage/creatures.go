package storage

import (
	"context"
	"fmt"
	"time"
)

type CreatureStats struct {
	ID              int64
	EntityID        int64
	CreatureType    string // beast, undead, fiend, dragon, aberration, etc.
	ChallengeRating string // "1/4", "3", "20"
	ArmorClass      *int
	HitPoints       string // "52 (8d8+16)"
	Abilities       string
	Loot            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreatureCombatStats struct {
	TotalEncounters  int      `json:"total_encounters"`
	TotalDamageDealt int      `json:"total_damage_dealt"`
	TotalDamageTaken int      `json:"total_damage_taken"`
	DefeatedBy       []string `json:"defeated_by"`
}

// UpsertCreatureStats inserts or updates creature-specific metadata for an entity.
func (s *Store) UpsertCreatureStats(ctx context.Context, entityID int64, stats CreatureStats) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO creature_stats (entity_id, creature_type, challenge_rating, armor_class, hit_points, abilities, loot)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (entity_id) DO UPDATE SET
			creature_type = EXCLUDED.creature_type,
			challenge_rating = EXCLUDED.challenge_rating,
			armor_class = EXCLUDED.armor_class,
			hit_points = EXCLUDED.hit_points,
			abilities = EXCLUDED.abilities,
			loot = EXCLUDED.loot,
			updated_at = NOW()`,
		entityID, stats.CreatureType, stats.ChallengeRating, stats.ArmorClass, stats.HitPoints, stats.Abilities, stats.Loot,
	)
	if err != nil {
		return fmt.Errorf("upsert creature stats: %w", err)
	}
	return nil
}

// GetCreatureStats returns creature-specific metadata for an entity.
func (s *Store) GetCreatureStats(ctx context.Context, entityID int64) (*CreatureStats, error) {
	var cs CreatureStats
	err := s.Pool.QueryRow(ctx,
		`SELECT id, entity_id, creature_type, challenge_rating, armor_class, hit_points, abilities, loot, created_at, updated_at
		 FROM creature_stats WHERE entity_id = $1`, entityID,
	).Scan(&cs.ID, &cs.EntityID, &cs.CreatureType, &cs.ChallengeRating, &cs.ArmorClass, &cs.HitPoints, &cs.Abilities, &cs.Loot, &cs.CreatedAt, &cs.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get creature stats: %w", err)
	}
	return &cs, nil
}

// ListCreaturesWithStats returns all creature entities for a campaign with their stats.
func (s *Store) ListCreaturesWithStats(ctx context.Context, campaignID int64, creatureTypeFilter, search string, limit, offset int) ([]Entity, []CreatureStats, error) {
	query := `SELECT e.id, e.campaign_id, e.name, e.type, e.description, e.status, e.cause_of_death, e.parent_entity_id, e.created_at, e.updated_at,
	                  COALESCE(cs.id, 0), COALESCE(cs.creature_type, ''), COALESCE(cs.challenge_rating, ''), cs.armor_class, COALESCE(cs.hit_points, ''), COALESCE(cs.abilities, ''), COALESCE(cs.loot, ''), COALESCE(cs.created_at, e.created_at), COALESCE(cs.updated_at, e.updated_at)
	           FROM entities e
	           LEFT JOIN creature_stats cs ON cs.entity_id = e.id
	           WHERE e.campaign_id = $1 AND e.type = 'creature'`
	args := []any{campaignID}
	argN := 2

	if creatureTypeFilter != "" {
		query += fmt.Sprintf(" AND cs.creature_type = $%d", argN)
		args = append(args, creatureTypeFilter)
		argN++
	}
	if search != "" {
		query += fmt.Sprintf(" AND e.name ILIKE $%d", argN)
		args = append(args, "%"+search+"%")
		argN++
	}

	query += " ORDER BY e.name"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argN, argN+1)
	args = append(args, limit, offset)

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list creatures: %w", err)
	}
	defer rows.Close()

	var entities []Entity
	var stats []CreatureStats
	for rows.Next() {
		var e Entity
		var cs CreatureStats
		if err := rows.Scan(
			&e.ID, &e.CampaignID, &e.Name, &e.Type, &e.Description, &e.Status, &e.CauseOfDeath, &e.ParentEntityID, &e.CreatedAt, &e.UpdatedAt,
			&cs.ID, &cs.CreatureType, &cs.ChallengeRating, &cs.ArmorClass, &cs.HitPoints, &cs.Abilities, &cs.Loot, &cs.CreatedAt, &cs.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		cs.EntityID = e.ID
		entities = append(entities, e)
		stats = append(stats, cs)
	}
	return entities, stats, rows.Err()
}

// GetCreatureCombatStats computes aggregate combat statistics for a creature entity
// from linked combat actions.
func (s *Store) GetCreatureCombatStats(ctx context.Context, entityID int64) (*CreatureCombatStats, error) {
	var stats CreatureCombatStats

	// Count distinct encounters where this creature participated.
	err := s.Pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT ca.encounter_id)
		 FROM combat_actions ca
		 WHERE ca.actor_entity_id = $1 OR ca.target_entity_id = $1`, entityID,
	).Scan(&stats.TotalEncounters)
	if err != nil {
		return nil, fmt.Errorf("creature combat stats (encounters): %w", err)
	}

	// Total damage dealt by this creature.
	s.Pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(damage), 0) FROM combat_actions WHERE actor_entity_id = $1 AND damage IS NOT NULL`, entityID,
	).Scan(&stats.TotalDamageDealt)

	// Total damage taken by this creature.
	s.Pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(damage), 0) FROM combat_actions WHERE target_entity_id = $1 AND damage IS NOT NULL`, entityID,
	).Scan(&stats.TotalDamageTaken)

	// Who defeated this creature (actors that dealt damage to it).
	rows, err := s.Pool.Query(ctx,
		`SELECT DISTINCT ca.actor FROM combat_actions ca
		 WHERE ca.target_entity_id = $1 AND ca.action_type = 'attack' AND ca.damage IS NOT NULL
		 ORDER BY ca.actor`, entityID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			if rows.Scan(&name) == nil {
				stats.DefeatedBy = append(stats.DefeatedBy, name)
			}
		}
	}

	return &stats, nil
}

// GetCreatureEncounterHistory returns all combat encounters where a creature participated.
func (s *Store) GetCreatureEncounterHistory(ctx context.Context, entityID int64) ([]CombatEncounter, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT DISTINCT ce.id, ce.session_id, ce.campaign_id, ce.name, ce.start_time, ce.end_time, ce.summary, ce.created_at
		 FROM combat_encounters ce
		 JOIN combat_actions ca ON ca.encounter_id = ce.id
		 WHERE ca.actor_entity_id = $1 OR ca.target_entity_id = $1
		 ORDER BY ce.created_at`, entityID)
	if err != nil {
		return nil, fmt.Errorf("creature encounter history: %w", err)
	}
	defer rows.Close()

	var encounters []CombatEncounter
	for rows.Next() {
		var e CombatEncounter
		if err := rows.Scan(&e.ID, &e.SessionID, &e.CampaignID, &e.Name, &e.StartTime, &e.EndTime, &e.Summary, &e.CreatedAt); err != nil {
			return nil, err
		}
		encounters = append(encounters, e)
	}
	return encounters, rows.Err()
}

// LinkCombatActorsToEntities updates combat_actions to link actor/target names to entity IDs
// for a given campaign's encounters.
func (s *Store) LinkCombatActorsToEntities(ctx context.Context, campaignID int64, nameToEntityID map[string]int64) error {
	for name, entityID := range nameToEntityID {
		_, err := s.Pool.Exec(ctx,
			`UPDATE combat_actions SET actor_entity_id = $1
			 WHERE actor = $2 AND encounter_id IN (
				SELECT id FROM combat_encounters WHERE campaign_id = $3
			 ) AND actor_entity_id IS NULL`, entityID, name, campaignID)
		if err != nil {
			return fmt.Errorf("link actor %q: %w", name, err)
		}

		_, err = s.Pool.Exec(ctx,
			`UPDATE combat_actions SET target_entity_id = $1
			 WHERE target = $2 AND encounter_id IN (
				SELECT id FROM combat_encounters WHERE campaign_id = $3
			 ) AND target_entity_id IS NULL`, entityID, name, campaignID)
		if err != nil {
			return fmt.Errorf("link target %q: %w", name, err)
		}
	}
	return nil
}

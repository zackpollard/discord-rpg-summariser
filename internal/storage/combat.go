package storage

import (
	"context"
	"fmt"
	"time"
)

// CombatEncounter represents a detected combat encounter within a session.
type CombatEncounter struct {
	ID         int64
	SessionID  int64
	CampaignID int64
	Name       string
	StartTime  float64
	EndTime    float64
	Summary    string
	CreatedAt  time.Time
}

// CombatAction represents a single action taken during a combat encounter.
type CombatAction struct {
	ID          int64
	EncounterID int64
	Actor       string
	ActionType  string // attack, spell, ability, heal, damage_taken, save, skill
	Target      string
	Detail      string
	Damage      *int
	Round       *int
	Timestamp   *float64
	CreatedAt   time.Time
}

// InsertCombatEncounter inserts a combat encounter and returns its ID.
func (s *Store) InsertCombatEncounter(ctx context.Context, enc CombatEncounter) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO combat_encounters (session_id, campaign_id, name, start_time, end_time, summary)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		enc.SessionID, enc.CampaignID, enc.Name, enc.StartTime, enc.EndTime, enc.Summary,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert combat encounter: %w", err)
	}
	return id, nil
}

// InsertCombatActions inserts a batch of combat actions for a given encounter.
func (s *Store) InsertCombatActions(ctx context.Context, encounterID int64, actions []CombatAction) error {
	for _, a := range actions {
		_, err := s.Pool.Exec(ctx,
			`INSERT INTO combat_actions (encounter_id, actor, action_type, target, detail, damage, round, timestamp)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			encounterID, a.Actor, a.ActionType, a.Target, a.Detail, a.Damage, a.Round, a.Timestamp,
		)
		if err != nil {
			return fmt.Errorf("insert combat action: %w", err)
		}
	}
	return nil
}

// GetCombatEncounters returns all combat encounters for a session.
func (s *Store) GetCombatEncounters(ctx context.Context, sessionID int64) ([]CombatEncounter, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, session_id, campaign_id, name, start_time, end_time, summary, created_at
		 FROM combat_encounters WHERE session_id = $1 ORDER BY start_time`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get combat encounters: %w", err)
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

// GetCombatActions returns all actions for a given combat encounter.
func (s *Store) GetCombatActions(ctx context.Context, encounterID int64) ([]CombatAction, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, encounter_id, actor, action_type, target, detail, damage, round, timestamp, created_at
		 FROM combat_actions WHERE encounter_id = $1 ORDER BY round NULLS LAST, timestamp NULLS LAST, id`, encounterID)
	if err != nil {
		return nil, fmt.Errorf("get combat actions: %w", err)
	}
	defer rows.Close()

	var actions []CombatAction
	for rows.Next() {
		var a CombatAction
		if err := rows.Scan(&a.ID, &a.EncounterID, &a.Actor, &a.ActionType, &a.Target, &a.Detail, &a.Damage, &a.Round, &a.Timestamp, &a.CreatedAt); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

// DeleteCombatForSession removes all combat encounters (and their cascaded
// actions) for a session. Used during reprocessing.
func (s *Store) DeleteCombatForSession(ctx context.Context, sessionID int64) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM combat_encounters WHERE session_id = $1`, sessionID)
	if err != nil {
		return fmt.Errorf("delete combat for session: %w", err)
	}
	return nil
}

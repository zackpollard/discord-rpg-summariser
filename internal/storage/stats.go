package storage

import (
	"context"
	"fmt"
	"time"
)

// CampaignStats holds all aggregate statistics for a campaign dashboard.
type CampaignStats struct {
	// Session stats
	TotalSessions  int     `json:"total_sessions"`
	TotalDurationMin float64 `json:"total_duration_min"`
	AvgDurationMin float64 `json:"avg_duration_min"`

	// Transcript stats
	TotalSegments int `json:"total_segments"`
	TotalWords    int `json:"total_words"`

	// Per-speaker stats (for bar chart)
	SpeakerStats []SpeakerStat `json:"speaker_stats"`

	// Entity stats
	EntityCounts map[string]int `json:"entity_counts"` // type -> count
	TopEntities  []TopEntity    `json:"top_entities"`  // most mentioned entities

	// Quest stats
	TotalQuests     int `json:"total_quests"`
	ActiveQuests    int `json:"active_quests"`
	CompletedQuests int `json:"completed_quests"`
	FailedQuests    int `json:"failed_quests"`

	// Combat stats
	TotalEncounters int               `json:"total_encounters"`
	TotalActions    int               `json:"total_actions"`
	TotalDamage     int               `json:"total_damage"`
	CombatActorStats []CombatActorStat `json:"combat_actor_stats"` // per-actor damage/action counts

	// Per-session data (for line charts)
	SessionTimeline []SessionTimelineStat `json:"session_timeline"`

	// NPC status breakdown
	NPCStatusCounts map[string]int `json:"npc_status_counts"` // alive/dead/unknown
}

// SpeakerStat holds per-speaker transcript statistics.
type SpeakerStat struct {
	UserID        string `json:"user_id"`
	CharacterName string `json:"character_name"`
	SegmentCount  int    `json:"segment_count"`
	WordCount     int    `json:"word_count"`
}

// TopEntity holds an entity with its mention count.
type TopEntity struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Mentions int    `json:"mentions"`
}

// CombatActorStat holds per-actor combat aggregates.
type CombatActorStat struct {
	Actor       string `json:"actor"`
	Actions     int    `json:"actions"`
	TotalDamage int    `json:"total_damage"`
}

// SessionTimelineStat holds per-session stats for timeline charts.
type SessionTimelineStat struct {
	SessionID    int64   `json:"session_id"`
	StartedAt    string  `json:"started_at"`
	DurationMin  float64 `json:"duration_min"`
	SegmentCount int     `json:"segment_count"`
	WordCount    int     `json:"word_count"`
}

// GetCampaignStats computes aggregate statistics for a campaign. It uses
// batched queries to minimise round-trips.
func (s *Store) GetCampaignStats(ctx context.Context, campaignID int64, guildID string) (*CampaignStats, error) {
	stats := &CampaignStats{
		EntityCounts:    make(map[string]int),
		NPCStatusCounts: make(map[string]int),
	}

	// 1. Session aggregates
	err := s.Pool.QueryRow(ctx,
		`SELECT COUNT(*),
		        COALESCE(SUM(EXTRACT(EPOCH FROM (ended_at - started_at)) / 60), 0),
		        COALESCE(AVG(EXTRACT(EPOCH FROM (ended_at - started_at)) / 60), 0)
		 FROM sessions
		 WHERE campaign_id = $1 AND guild_id = $2 AND status = 'complete' AND ended_at IS NOT NULL`,
		campaignID, guildID,
	).Scan(&stats.TotalSessions, &stats.TotalDurationMin, &stats.AvgDurationMin)
	if err != nil {
		return nil, fmt.Errorf("session aggregates: %w", err)
	}

	// 2. Transcript aggregates
	err = s.Pool.QueryRow(ctx,
		`SELECT COUNT(*),
		        COALESCE(SUM(array_length(regexp_split_to_array(trim(ts.text), '\s+'), 1)), 0)
		 FROM transcript_segments ts
		 JOIN sessions s ON s.id = ts.session_id
		 WHERE s.campaign_id = $1 AND s.guild_id = $2`,
		campaignID, guildID,
	).Scan(&stats.TotalSegments, &stats.TotalWords)
	if err != nil {
		return nil, fmt.Errorf("transcript aggregates: %w", err)
	}

	// 3. Per-speaker stats with character name join
	speakerRows, err := s.Pool.Query(ctx,
		`SELECT ts.user_id,
		        COALESCE(cm.character_name, ts.user_id) AS character_name,
		        COUNT(*) AS segment_count,
		        COALESCE(SUM(array_length(regexp_split_to_array(trim(ts.text), '\s+'), 1)), 0) AS word_count
		 FROM transcript_segments ts
		 JOIN sessions s ON s.id = ts.session_id
		 LEFT JOIN character_mappings cm ON cm.user_id = ts.user_id AND cm.campaign_id = s.campaign_id
		 WHERE s.campaign_id = $1 AND s.guild_id = $2
		 GROUP BY ts.user_id, cm.character_name
		 ORDER BY word_count DESC`,
		campaignID, guildID,
	)
	if err != nil {
		return nil, fmt.Errorf("speaker stats: %w", err)
	}
	defer speakerRows.Close()

	for speakerRows.Next() {
		var ss SpeakerStat
		if err := speakerRows.Scan(&ss.UserID, &ss.CharacterName, &ss.SegmentCount, &ss.WordCount); err != nil {
			return nil, fmt.Errorf("scan speaker stat: %w", err)
		}
		stats.SpeakerStats = append(stats.SpeakerStats, ss)
	}
	if err := speakerRows.Err(); err != nil {
		return nil, fmt.Errorf("speaker rows: %w", err)
	}
	if stats.SpeakerStats == nil {
		stats.SpeakerStats = []SpeakerStat{}
	}

	// 4. Entity type counts
	entityRows, err := s.Pool.Query(ctx,
		`SELECT type, COUNT(*) FROM entities WHERE campaign_id = $1 GROUP BY type`,
		campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("entity counts: %w", err)
	}
	defer entityRows.Close()

	for entityRows.Next() {
		var typ string
		var count int
		if err := entityRows.Scan(&typ, &count); err != nil {
			return nil, fmt.Errorf("scan entity count: %w", err)
		}
		stats.EntityCounts[typ] = count
	}
	if err := entityRows.Err(); err != nil {
		return nil, fmt.Errorf("entity count rows: %w", err)
	}

	// 5. Top entities by mention count (top 10)
	topRows, err := s.Pool.Query(ctx,
		`SELECT e.name, e.type, COUNT(*) AS mentions
		 FROM entity_references er
		 JOIN entities e ON e.id = er.entity_id
		 WHERE e.campaign_id = $1
		 GROUP BY e.id, e.name, e.type
		 ORDER BY mentions DESC
		 LIMIT 10`,
		campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("top entities: %w", err)
	}
	defer topRows.Close()

	for topRows.Next() {
		var te TopEntity
		if err := topRows.Scan(&te.Name, &te.Type, &te.Mentions); err != nil {
			return nil, fmt.Errorf("scan top entity: %w", err)
		}
		stats.TopEntities = append(stats.TopEntities, te)
	}
	if err := topRows.Err(); err != nil {
		return nil, fmt.Errorf("top entity rows: %w", err)
	}
	if stats.TopEntities == nil {
		stats.TopEntities = []TopEntity{}
	}

	// 6. Quest stats
	err = s.Pool.QueryRow(ctx,
		`SELECT COUNT(*),
		        COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0)
		 FROM quests
		 WHERE campaign_id = $1`,
		campaignID,
	).Scan(&stats.TotalQuests, &stats.ActiveQuests, &stats.CompletedQuests, &stats.FailedQuests)
	if err != nil {
		return nil, fmt.Errorf("quest stats: %w", err)
	}

	// 7. Combat aggregates
	err = s.Pool.QueryRow(ctx,
		`SELECT COALESCE(enc_count, 0), COALESCE(act_count, 0), COALESCE(total_dmg, 0)
		 FROM (
		   SELECT COUNT(*) AS enc_count FROM combat_encounters WHERE campaign_id = $1
		 ) e,
		 (
		   SELECT COUNT(*) AS act_count, COALESCE(SUM(COALESCE(ca.damage, 0)), 0) AS total_dmg
		   FROM combat_actions ca
		   JOIN combat_encounters ce ON ce.id = ca.encounter_id
		   WHERE ce.campaign_id = $1
		 ) a`,
		campaignID,
	).Scan(&stats.TotalEncounters, &stats.TotalActions, &stats.TotalDamage)
	if err != nil {
		return nil, fmt.Errorf("combat aggregates: %w", err)
	}

	// 8. Combat actor stats
	actorRows, err := s.Pool.Query(ctx,
		`SELECT ca.actor, COUNT(*) AS actions, COALESCE(SUM(COALESCE(ca.damage, 0)), 0) AS total_damage
		 FROM combat_actions ca
		 JOIN combat_encounters ce ON ce.id = ca.encounter_id
		 WHERE ce.campaign_id = $1
		 GROUP BY ca.actor
		 ORDER BY total_damage DESC
		 LIMIT 20`,
		campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("combat actor stats: %w", err)
	}
	defer actorRows.Close()

	for actorRows.Next() {
		var cas CombatActorStat
		if err := actorRows.Scan(&cas.Actor, &cas.Actions, &cas.TotalDamage); err != nil {
			return nil, fmt.Errorf("scan combat actor: %w", err)
		}
		stats.CombatActorStats = append(stats.CombatActorStats, cas)
	}
	if err := actorRows.Err(); err != nil {
		return nil, fmt.Errorf("combat actor rows: %w", err)
	}
	if stats.CombatActorStats == nil {
		stats.CombatActorStats = []CombatActorStat{}
	}

	// 9. Per-session timeline data
	timelineRows, err := s.Pool.Query(ctx,
		`SELECT s.id,
		        s.started_at,
		        COALESCE(EXTRACT(EPOCH FROM (s.ended_at - s.started_at)) / 60, 0) AS duration_min,
		        COUNT(ts.id) AS segment_count,
		        COALESCE(SUM(array_length(regexp_split_to_array(trim(ts.text), '\s+'), 1)), 0) AS word_count
		 FROM sessions s
		 LEFT JOIN transcript_segments ts ON ts.session_id = s.id
		 WHERE s.campaign_id = $1 AND s.guild_id = $2 AND s.status = 'complete' AND s.ended_at IS NOT NULL
		 GROUP BY s.id, s.started_at, s.ended_at
		 ORDER BY s.started_at`,
		campaignID, guildID,
	)
	if err != nil {
		return nil, fmt.Errorf("session timeline: %w", err)
	}
	defer timelineRows.Close()

	for timelineRows.Next() {
		var st SessionTimelineStat
		var startedAt time.Time
		if err := timelineRows.Scan(&st.SessionID, &startedAt, &st.DurationMin, &st.SegmentCount, &st.WordCount); err != nil {
			return nil, fmt.Errorf("scan session timeline: %w", err)
		}
		st.StartedAt = startedAt.Format(time.RFC3339)
		stats.SessionTimeline = append(stats.SessionTimeline, st)
	}
	if err := timelineRows.Err(); err != nil {
		return nil, fmt.Errorf("session timeline rows: %w", err)
	}
	if stats.SessionTimeline == nil {
		stats.SessionTimeline = []SessionTimelineStat{}
	}

	// 10. NPC status breakdown
	npcRows, err := s.Pool.Query(ctx,
		`SELECT status, COUNT(*) FROM entities WHERE campaign_id = $1 AND type = 'npc' GROUP BY status`,
		campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("npc status counts: %w", err)
	}
	defer npcRows.Close()

	for npcRows.Next() {
		var status string
		var count int
		if err := npcRows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan npc status: %w", err)
		}
		stats.NPCStatusCounts[status] = count
	}
	if err := npcRows.Err(); err != nil {
		return nil, fmt.Errorf("npc status rows: %w", err)
	}

	return stats, nil
}

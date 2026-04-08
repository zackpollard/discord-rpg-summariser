package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type creatureStatsResponse struct {
	CreatureType    string `json:"creature_type"`
	ChallengeRating string `json:"challenge_rating"`
	ArmorClass      *int   `json:"armor_class"`
	HitPoints       string `json:"hit_points"`
	Abilities       string `json:"abilities"`
	Loot            string `json:"loot"`
}

type bestiaryEntryResponse struct {
	ID            int64                 `json:"id"`
	CampaignID    int64                 `json:"campaign_id"`
	Name          string                `json:"name"`
	Description   string                `json:"description"`
	Status        string                `json:"status"`
	CreatedAt     time.Time             `json:"created_at"`
	CreatureStats creatureStatsResponse `json:"creature_stats"`
}

type creatureDetailResponse struct {
	entityDetailResponse
	CreatureStats    *creatureStatsResponse           `json:"creature_stats"`
	CombatStats      *creatureCombatStatsResponse     `json:"combat_stats"`
	EncounterHistory []combatEncounterHistoryResponse `json:"encounter_history"`
}

type creatureCombatStatsResponse struct {
	TotalEncounters  int      `json:"total_encounters"`
	TotalDamageDealt int      `json:"total_damage_dealt"`
	TotalDamageTaken int      `json:"total_damage_taken"`
	DefeatedBy       []string `json:"defeated_by"`
}

type combatEncounterHistoryResponse struct {
	ID        int64     `json:"id"`
	SessionID int64     `json:"session_id"`
	Name      string    `json:"name"`
	StartTime float64   `json:"start_time"`
	EndTime   float64   `json:"end_time"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) handleListBestiary(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	creatureType := r.URL.Query().Get("creature_type")
	search := r.URL.Query().Get("search")
	limit, offset := parsePagination(r, 50)

	entities, stats, err := s.store.ListCreaturesWithStats(r.Context(), campaignID, creatureType, search, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list creatures")
		return
	}

	resp := make([]bestiaryEntryResponse, len(entities))
	for i := range entities {
		resp[i] = bestiaryEntryResponse{
			ID:          entities[i].ID,
			CampaignID:  entities[i].CampaignID,
			Name:        entities[i].Name,
			Description: entities[i].Description,
			Status:      entities[i].Status,
			CreatedAt:   entities[i].CreatedAt,
			CreatureStats: creatureStatsResponse{
				CreatureType:    stats[i].CreatureType,
				ChallengeRating: stats[i].ChallengeRating,
				ArmorClass:      stats[i].ArmorClass,
				HitPoints:       stats[i].HitPoints,
				Abilities:       stats[i].Abilities,
				Loot:            stats[i].Loot,
			},
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetCreature(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	entity, err := s.store.GetEntity(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "creature not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get creature")
		return
	}

	// Fetch entity detail data.
	notes, _ := s.store.GetEntityNotes(r.Context(), id)
	rels, _ := s.store.GetEntityRelationships(r.Context(), id)
	appearances, _ := s.store.GetEntitySessionAppearances(r.Context(), id)
	refs, _ := s.store.GetEntityReferences(r.Context(), id, 100, 0)

	// Build entity name cache for relationships.
	entityNames := map[int64]string{entity.ID: entity.Name}
	for _, rel := range rels {
		for _, eid := range []int64{rel.SourceID, rel.TargetID} {
			if _, ok := entityNames[eid]; !ok {
				if e, err := s.store.GetEntity(r.Context(), eid); err == nil {
					entityNames[eid] = e.Name
				}
			}
		}
	}

	// Build entity detail base.
	detail := entityDetailResponse{
		entityResponse: entityResponse{
			ID:             entity.ID,
			CampaignID:     entity.CampaignID,
			Name:           entity.Name,
			Type:           entity.Type,
			Description:    entity.Description,
			Status:         entity.Status,
			CauseOfDeath:   entity.CauseOfDeath,
			ParentEntityID: entity.ParentEntityID,
			CreatedAt:      entity.CreatedAt,
			UpdatedAt:      entity.UpdatedAt,
		},
	}

	for _, n := range notes {
		detail.Notes = append(detail.Notes, entityNoteResponse{
			ID:        n.ID,
			SessionID: n.SessionID,
			Content:   n.Content,
			CreatedAt: n.CreatedAt,
		})
	}
	for _, rel := range rels {
		detail.Relationships = append(detail.Relationships, entityRelationshipResponse{
			ID:           rel.ID,
			SourceID:     rel.SourceID,
			SourceName:   entityNames[rel.SourceID],
			TargetID:     rel.TargetID,
			TargetName:   entityNames[rel.TargetID],
			Relationship: rel.Relationship,
			Description:  rel.Description,
		})
	}
	for _, a := range appearances {
		detail.Sessions = append(detail.Sessions, entitySessionResponse{
			SessionID:    a.SessionID,
			StartedAt:    a.StartedAt,
			MentionCount: a.MentionCount,
		})
	}
	for _, ref := range refs {
		detail.References = append(detail.References, entityReferenceResponse{
			SessionID: ref.SessionID,
			SegmentID: ref.SegmentID,
			StartTime: ref.StartTime,
			Context:   ref.Context,
		})
	}

	// Creature-specific data.
	resp := creatureDetailResponse{entityDetailResponse: detail}

	cs, err := s.store.GetCreatureStats(r.Context(), id)
	if err == nil {
		resp.CreatureStats = &creatureStatsResponse{
			CreatureType:    cs.CreatureType,
			ChallengeRating: cs.ChallengeRating,
			ArmorClass:      cs.ArmorClass,
			HitPoints:       cs.HitPoints,
			Abilities:       cs.Abilities,
			Loot:            cs.Loot,
		}
	}

	combatStats, err := s.store.GetCreatureCombatStats(r.Context(), id)
	if err == nil {
		defeated := combatStats.DefeatedBy
		if defeated == nil {
			defeated = []string{}
		}
		resp.CombatStats = &creatureCombatStatsResponse{
			TotalEncounters:  combatStats.TotalEncounters,
			TotalDamageDealt: combatStats.TotalDamageDealt,
			TotalDamageTaken: combatStats.TotalDamageTaken,
			DefeatedBy:       defeated,
		}
	}

	encounterHistory, err := s.store.GetCreatureEncounterHistory(r.Context(), id)
	if err == nil {
		for _, enc := range encounterHistory {
			resp.EncounterHistory = append(resp.EncounterHistory, combatEncounterHistoryResponse{
				ID:        enc.ID,
				SessionID: enc.SessionID,
				Name:      enc.Name,
				StartTime: enc.StartTime,
				EndTime:   enc.EndTime,
				Summary:   enc.Summary,
				CreatedAt: enc.CreatedAt,
			})
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type entityResponse struct {
	ID          int64     `json:"id"`
	CampaignID  int64     `json:"campaign_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type entityDetailResponse struct {
	entityResponse
	Notes         []entityNoteResponse         `json:"notes"`
	Relationships []entityRelationshipResponse `json:"relationships"`
}

type entityNoteResponse struct {
	ID        int64     `json:"id"`
	SessionID int64     `json:"session_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type entityRelationshipResponse struct {
	ID           int64  `json:"id"`
	SourceID     int64  `json:"source_id"`
	SourceName   string `json:"source_name"`
	TargetID     int64  `json:"target_id"`
	TargetName   string `json:"target_name"`
	Relationship string `json:"relationship"`
	Description  string `json:"description"`
}

func (s *Server) handleListEntities(w http.ResponseWriter, r *http.Request) {
	campaignID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
		return
	}

	typeFilter := r.URL.Query().Get("type")
	search := r.URL.Query().Get("search")

	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	entities, err := s.store.ListEntities(r.Context(), campaignID, typeFilter, search, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list entities")
		return
	}

	resp := make([]entityResponse, len(entities))
	for i := range entities {
		e := &entities[i]
		resp[i] = entityResponse{
			ID:          e.ID,
			CampaignID:  e.CampaignID,
			Name:        e.Name,
			Type:        e.Type,
			Description: e.Description,
			CreatedAt:   e.CreatedAt,
			UpdatedAt:   e.UpdatedAt,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetEntity(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid entity id")
		return
	}

	entity, err := s.store.GetEntity(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "entity not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get entity")
		return
	}

	notes, err := s.store.GetEntityNotes(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get entity notes")
		return
	}

	rels, err := s.store.GetEntityRelationships(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get entity relationships")
		return
	}

	// Build a cache of entity IDs to names for relationship resolution
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

	noteResp := make([]entityNoteResponse, len(notes))
	for i := range notes {
		n := &notes[i]
		noteResp[i] = entityNoteResponse{
			ID:        n.ID,
			SessionID: n.SessionID,
			Content:   n.Content,
			CreatedAt: n.CreatedAt,
		}
	}

	relResp := make([]entityRelationshipResponse, len(rels))
	for i := range rels {
		rel := &rels[i]
		relResp[i] = entityRelationshipResponse{
			ID:           rel.ID,
			SourceID:     rel.SourceID,
			SourceName:   entityNames[rel.SourceID],
			TargetID:     rel.TargetID,
			TargetName:   entityNames[rel.TargetID],
			Relationship: rel.Relationship,
			Description:  rel.Description,
		}
	}

	writeJSON(w, http.StatusOK, entityDetailResponse{
		entityResponse: entityResponse{
			ID:          entity.ID,
			CampaignID:  entity.CampaignID,
			Name:        entity.Name,
			Type:        entity.Type,
			Description: entity.Description,
			CreatedAt:   entity.CreatedAt,
			UpdatedAt:   entity.UpdatedAt,
		},
		Notes:         noteResp,
		Relationships: relResp,
	})
}

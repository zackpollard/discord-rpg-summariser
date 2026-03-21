package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type entityResponse struct {
	ID           int64     `json:"id"`
	CampaignID   int64     `json:"campaign_id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	CauseOfDeath string    `json:"cause_of_death"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type entityDetailResponse struct {
	entityResponse
	Notes         []entityNoteResponse         `json:"notes"`
	Relationships []entityRelationshipResponse `json:"relationships"`
	Sessions      []entitySessionResponse      `json:"sessions"`
	References    []entityReferenceResponse    `json:"references"`
}

type entitySessionResponse struct {
	SessionID    int64     `json:"session_id"`
	StartedAt    time.Time `json:"started_at"`
	MentionCount int       `json:"mention_count"`
}

type entityReferenceResponse struct {
	SessionID int64   `json:"session_id"`
	SegmentID *int64  `json:"segment_id"`
	StartTime float64 `json:"start_time"`
	Context   string  `json:"context"`
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
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	typeFilter := r.URL.Query().Get("type")
	search := r.URL.Query().Get("search")
	statusFilter := r.URL.Query().Get("status")
	limit, offset := parsePagination(r, 50)

	entities, err := s.store.ListEntities(r.Context(), campaignID, typeFilter, search, limit, offset, statusFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list entities")
		return
	}

	resp := make([]entityResponse, len(entities))
	for i := range entities {
		e := &entities[i]
		resp[i] = entityResponse{
			ID:           e.ID,
			CampaignID:   e.CampaignID,
			Name:         e.Name,
			Type:         e.Type,
			Description:  e.Description,
			Status:       e.Status,
			CauseOfDeath: e.CauseOfDeath,
			CreatedAt:    e.CreatedAt,
			UpdatedAt:    e.UpdatedAt,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetEntity(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
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

	// Fetch session appearances and references.
	appearances, err := s.store.GetEntitySessionAppearances(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get entity session appearances")
		return
	}

	refs, err := s.store.GetEntityReferences(r.Context(), id, 100, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get entity references")
		return
	}

	sessResp := make([]entitySessionResponse, len(appearances))
	for i := range appearances {
		a := &appearances[i]
		sessResp[i] = entitySessionResponse{
			SessionID:    a.SessionID,
			StartedAt:    a.StartedAt,
			MentionCount: a.MentionCount,
		}
	}

	refResp := make([]entityReferenceResponse, len(refs))
	for i := range refs {
		ref := &refs[i]
		refResp[i] = entityReferenceResponse{
			SessionID: ref.SessionID,
			SegmentID: ref.SegmentID,
			StartTime: ref.StartTime,
			Context:   ref.Context,
		}
	}

	writeJSON(w, http.StatusOK, entityDetailResponse{
		entityResponse: entityResponse{
			ID:           entity.ID,
			CampaignID:   entity.CampaignID,
			Name:         entity.Name,
			Type:         entity.Type,
			Description:  entity.Description,
			Status:       entity.Status,
			CauseOfDeath: entity.CauseOfDeath,
			CreatedAt:    entity.CreatedAt,
			UpdatedAt:    entity.UpdatedAt,
		},
		Notes:         noteResp,
		Relationships: relResp,
		Sessions:      sessResp,
		References:    refResp,
	})
}

func (s *Server) handleMergeEntity(w http.ResponseWriter, r *http.Request) {
	keepID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	var body struct {
		MergeID int64 `json:"merge_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.MergeID == 0 {
		writeError(w, http.StatusBadRequest, "merge_id is required")
		return
	}
	if body.MergeID == keepID {
		writeError(w, http.StatusBadRequest, "cannot merge an entity with itself")
		return
	}

	// Validate both entities exist and belong to the same campaign.
	keepEntity, err := s.store.GetEntity(r.Context(), keepID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "kept entity not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get kept entity")
		return
	}

	mergeEntity, err := s.store.GetEntity(r.Context(), body.MergeID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "merge entity not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get merge entity")
		return
	}

	if keepEntity.CampaignID != mergeEntity.CampaignID {
		writeError(w, http.StatusBadRequest, "entities must belong to the same campaign")
		return
	}

	if err := s.store.MergeEntities(r.Context(), keepEntity.CampaignID, keepID, body.MergeID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to merge entities")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

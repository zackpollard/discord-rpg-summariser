package api

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
)

type characterSummaryResponse struct {
	StoryArc              string                `json:"story_arc"`
	KeyMoments            []string              `json:"key_moments"`
	RelationshipSummaries []relationshipNoteAPI `json:"relationship_summaries"`
}

type relationshipNoteAPI struct {
	Character string `json:"character"`
	Summary   string `json:"summary"`
}

func (s *Server) handleGetCharacterSummary(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	if s.summariser == nil {
		writeError(w, http.StatusServiceUnavailable, "summariser not available")
		return
	}

	userID := r.PathValue("userId")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "invalid userId")
		return
	}

	// Resolve character name.
	characterName, err := s.store.GetCharacterName(r.Context(), userID, campaignID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve character")
		return
	}
	if characterName == "" {
		writeError(w, http.StatusNotFound, "character not found")
		return
	}

	campaign, err := s.store.GetCampaign(r.Context(), campaignID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "campaign not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get campaign")
		return
	}

	// Get all session summaries.
	sessions, err := s.store.ListSessions(r.Context(), campaign.GuildID, campaignID, 100, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	var summaries []string
	for idx := len(sessions) - 1; idx >= 0; idx-- {
		sess := sessions[idx]
		if sess.Status == "complete" && sess.Summary != nil && *sess.Summary != "" {
			summaries = append(summaries, *sess.Summary)
		}
	}

	if len(summaries) == 0 {
		writeError(w, http.StatusNotFound, "no completed sessions found")
		return
	}

	// Get entity relationships for this character.
	entities, err := s.store.ListEntities(r.Context(), campaignID, "", characterName, 10, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search entities")
		return
	}

	var relationships []string
	for _, entity := range entities {
		rels, err := s.store.GetEntityRelationships(r.Context(), entity.ID)
		if err != nil {
			continue
		}
		for _, rel := range rels {
			// Resolve names.
			srcName := fmt.Sprintf("Entity#%d", rel.SourceID)
			tgtName := fmt.Sprintf("Entity#%d", rel.TargetID)
			if src, err := s.store.GetEntity(r.Context(), rel.SourceID); err == nil {
				srcName = src.Name
			}
			if tgt, err := s.store.GetEntity(r.Context(), rel.TargetID); err == nil {
				tgtName = tgt.Name
			}
			relationships = append(relationships, fmt.Sprintf("%s %s %s: %s", srcName, rel.Relationship, tgtName, rel.Description))
		}
	}

	result, err := s.summariser.GenerateCharacterSummary(r.Context(), characterName, summaries, relationships)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate character summary")
		return
	}

	resp := characterSummaryResponse{
		StoryArc:   result.StoryArc,
		KeyMoments: result.KeyMoments,
	}
	if resp.KeyMoments == nil {
		resp.KeyMoments = []string{}
	}
	resp.RelationshipSummaries = make([]relationshipNoteAPI, len(result.RelationshipSummaries))
	for i, rs := range result.RelationshipSummaries {
		resp.RelationshipSummaries[i] = relationshipNoteAPI{
			Character: rs.Character,
			Summary:   rs.Summary,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

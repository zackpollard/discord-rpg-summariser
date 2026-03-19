package api

import (
	"encoding/json"
	"net/http"
	"time"

	"discord-rpg-summariser/internal/storage"
)

type characterResponse struct {
	UserID        string    `json:"user_id"`
	GuildID       string    `json:"guild_id"`
	CampaignID    int64     `json:"campaign_id"`
	CharacterName string    `json:"character_name"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type upsertCharacterRequest struct {
	UserID        string `json:"user_id"`
	CharacterName string `json:"character_name"`
}

func toCharacterResponse(m *storage.CharacterMapping) characterResponse {
	return characterResponse{
		UserID:        m.UserID,
		GuildID:       m.GuildID,
		CampaignID:    m.CampaignID,
		CharacterName: m.CharacterName,
		UpdatedAt:     m.UpdatedAt,
	}
}

func (s *Server) handleListCharacters(w http.ResponseWriter, r *http.Request) {
	campaign, err := s.store.GetOrCreateActiveCampaign(r.Context(), s.guildID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve active campaign")
		return
	}

	mappings, err := s.store.GetCharacterMappings(r.Context(), campaign.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list characters")
		return
	}

	resp := make([]characterResponse, len(mappings))
	for i := range mappings {
		resp[i] = toCharacterResponse(&mappings[i])
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpsertCharacter(w http.ResponseWriter, r *http.Request) {
	var req upsertCharacterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" || req.CharacterName == "" {
		writeError(w, http.StatusBadRequest, "user_id and character_name are required")
		return
	}

	campaign, err := s.store.GetOrCreateActiveCampaign(r.Context(), s.guildID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve active campaign")
		return
	}

	mapping := storage.CharacterMapping{
		UserID:        req.UserID,
		GuildID:       s.guildID,
		CampaignID:    campaign.ID,
		CharacterName: req.CharacterName,
	}

	if err := s.store.SetCharacterMapping(r.Context(), mapping); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save character mapping")
		return
	}

	writeJSON(w, http.StatusOK, toCharacterResponse(&mapping))
}

func (s *Server) handleDeleteCharacter(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	campaign, err := s.store.GetOrCreateActiveCampaign(r.Context(), s.guildID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve active campaign")
		return
	}

	if err := s.store.DeleteCharacterMapping(r.Context(), userID, campaign.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete character mapping")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

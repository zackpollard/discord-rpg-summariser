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
	CharacterName string    `json:"character_name"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type upsertCharacterRequest struct {
	UserID        string `json:"user_id"`
	GuildID       string `json:"guild_id"`
	CharacterName string `json:"character_name"`
}

func toCharacterResponse(m *storage.CharacterMapping) characterResponse {
	return characterResponse{
		UserID:        m.UserID,
		GuildID:       m.GuildID,
		CharacterName: m.CharacterName,
		UpdatedAt:     m.UpdatedAt,
	}
}

func (s *Server) handleListCharacters(w http.ResponseWriter, r *http.Request) {
	guildID := r.URL.Query().Get("guild_id")
	if guildID == "" {
		guildID = s.guildID
	}

	mappings, err := s.store.GetCharacterMappings(r.Context(), guildID)
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

	guildID := req.GuildID
	if guildID == "" {
		guildID = s.guildID
	}

	mapping := storage.CharacterMapping{
		UserID:        req.UserID,
		GuildID:       guildID,
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

	guildID := r.URL.Query().Get("guild_id")
	if guildID == "" {
		guildID = s.guildID
	}

	if err := s.store.DeleteCharacterMapping(r.Context(), userID, guildID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete character mapping")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

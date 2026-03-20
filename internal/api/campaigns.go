package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"discord-rpg-summariser/internal/storage"

	"github.com/jackc/pgx/v5"
)

type campaignResponse struct {
	ID               int64      `json:"id"`
	GuildID          string     `json:"guild_id"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	IsActive         bool       `json:"is_active"`
	DMUserID         *string    `json:"dm_user_id"`
	Recap            string     `json:"recap"`
	RecapGeneratedAt *time.Time `json:"recap_generated_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

func toCampaignResponse(c *storage.Campaign) campaignResponse {
	return campaignResponse{
		ID: c.ID, GuildID: c.GuildID, Name: c.Name,
		Description: c.Description, IsActive: c.IsActive,
		DMUserID: c.DMUserID, Recap: c.Recap,
		RecapGeneratedAt: c.RecapGeneratedAt, CreatedAt: c.CreatedAt,
	}
}

type createCampaignRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleListCampaigns(w http.ResponseWriter, r *http.Request) {
	campaigns, err := s.store.ListCampaigns(r.Context(), s.guildID)
	if err != nil {
		log.Printf("ListCampaigns error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list campaigns")
		return
	}

	resp := make([]campaignResponse, len(campaigns))
	for i := range campaigns {
		resp[i] = toCampaignResponse(&campaigns[i])
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreateCampaign(w http.ResponseWriter, r *http.Request) {
	var req createCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	id, err := s.store.CreateCampaign(r.Context(), s.guildID, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create campaign")
		return
	}

	campaign, err := s.store.GetCampaign(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get created campaign")
		return
	}

	writeJSON(w, http.StatusCreated, toCampaignResponse(campaign))
}

func (s *Server) handleGetCampaign(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	campaign, err := s.store.GetCampaign(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "campaign not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get campaign")
		return
	}

	writeJSON(w, http.StatusOK, toCampaignResponse(campaign))
}

func (s *Server) handleSetActiveCampaign(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	if err := s.store.SetActiveCampaign(r.Context(), s.guildID, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set active campaign")
		return
	}

	campaign, err := s.store.GetCampaign(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get campaign")
		return
	}

	writeJSON(w, http.StatusOK, toCampaignResponse(campaign))
}

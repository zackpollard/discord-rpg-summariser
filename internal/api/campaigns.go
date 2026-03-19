package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type campaignResponse struct {
	ID          int64     `json:"id"`
	GuildID     string    `json:"guild_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type createCampaignRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleListCampaigns(w http.ResponseWriter, r *http.Request) {
	campaigns, err := s.store.ListCampaigns(r.Context(), s.guildID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list campaigns")
		return
	}

	resp := make([]campaignResponse, len(campaigns))
	for i := range campaigns {
		c := &campaigns[i]
		resp[i] = campaignResponse{
			ID:          c.ID,
			GuildID:     c.GuildID,
			Name:        c.Name,
			Description: c.Description,
			IsActive:    c.IsActive,
			CreatedAt:   c.CreatedAt,
		}
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

	writeJSON(w, http.StatusCreated, campaignResponse{
		ID:          campaign.ID,
		GuildID:     campaign.GuildID,
		Name:        campaign.Name,
		Description: campaign.Description,
		IsActive:    campaign.IsActive,
		CreatedAt:   campaign.CreatedAt,
	})
}

func (s *Server) handleGetCampaign(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
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

	writeJSON(w, http.StatusOK, campaignResponse{
		ID:          campaign.ID,
		GuildID:     campaign.GuildID,
		Name:        campaign.Name,
		Description: campaign.Description,
		IsActive:    campaign.IsActive,
		CreatedAt:   campaign.CreatedAt,
	})
}

func (s *Server) handleSetActiveCampaign(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
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

	writeJSON(w, http.StatusOK, campaignResponse{
		ID:          campaign.ID,
		GuildID:     campaign.GuildID,
		Name:        campaign.Name,
		Description: campaign.Description,
		IsActive:    campaign.IsActive,
		CreatedAt:   campaign.CreatedAt,
	})
}

package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type recapResponse struct {
	CampaignID       int64      `json:"campaign_id"`
	Recap            string     `json:"recap"`
	RecapGeneratedAt *time.Time `json:"recap_generated_at"`
}

func (s *Server) handleGetRecap(w http.ResponseWriter, r *http.Request) {
	campaignID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
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

	writeJSON(w, http.StatusOK, recapResponse{
		CampaignID:       campaign.ID,
		Recap:            campaign.Recap,
		RecapGeneratedAt: campaign.RecapGeneratedAt,
	})
}

func (s *Server) handleRegenerateRecap(w http.ResponseWriter, r *http.Request) {
	campaignID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
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

	if s.loreQA == nil {
		writeError(w, http.StatusServiceUnavailable, "recap generation provider not available")
		return
	}

	// Gather session summaries as context for recap generation.
	sessions, err := s.store.ListSessions(r.Context(), campaign.GuildID, campaignID, 100, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	var context string
	for _, sess := range sessions {
		if sess.Summary != nil {
			context += "Session #" + strconv.FormatInt(sess.ID, 10) + ": " + *sess.Summary + "\n\n"
		}
	}

	recap, err := s.loreQA.AskLore(r.Context(), campaignID,
		"Generate a comprehensive story recap for this campaign based on the session summaries.", context)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate recap")
		return
	}

	if err := s.store.UpdateCampaignRecap(r.Context(), campaignID, recap); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save recap")
		return
	}

	now := time.Now()
	writeJSON(w, http.StatusOK, recapResponse{
		CampaignID:       campaignID,
		Recap:            recap,
		RecapGeneratedAt: &now,
	})
}

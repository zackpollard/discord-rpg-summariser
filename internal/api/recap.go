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
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
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
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
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

	// Check for optional "last" query parameter.
	var lastN int
	if v := r.URL.Query().Get("last"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "last must be a positive integer")
			return
		}
		lastN = n
	}

	// Gather session summaries as context for recap generation.
	var summaryContext string
	if lastN > 0 {
		sessions, err := s.store.GetLatestCompleteSessions(r.Context(), campaignID, lastN)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list sessions")
			return
		}
		for _, sess := range sessions {
			if sess.Summary != nil {
				summaryContext += "Session #" + strconv.FormatInt(sess.ID, 10) + ": " + *sess.Summary + "\n\n"
			}
		}
	} else {
		sessions, err := s.store.ListSessions(r.Context(), campaign.GuildID, campaignID, 100, 0)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list sessions")
			return
		}
		for _, sess := range sessions {
			if sess.Summary != nil {
				summaryContext += "Session #" + strconv.FormatInt(sess.ID, 10) + ": " + *sess.Summary + "\n\n"
			}
		}
	}

	var prompt string
	if lastN > 0 {
		prompt = "Generate a narrative recap of the most recent " + strconv.Itoa(lastN) + " sessions based on the session summaries."
	} else {
		prompt = "Generate a comprehensive story recap for this campaign based on the session summaries."
	}

	recap, err := s.loreQA.AskLore(r.Context(), campaignID, prompt, summaryContext)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate recap")
		return
	}

	// Only persist to campaigns.recap when generating a full recap (no "last" filter).
	if lastN == 0 {
		if err := s.store.UpdateCampaignRecap(r.Context(), campaignID, recap); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save recap")
			return
		}
	}

	now := time.Now()
	writeJSON(w, http.StatusOK, recapResponse{
		CampaignID:       campaignID,
		Recap:            recap,
		RecapGeneratedAt: &now,
	})
}

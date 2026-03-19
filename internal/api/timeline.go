package api

import (
	"net/http"
	"strconv"

	"discord-rpg-summariser/internal/storage"
)

func (s *Server) handleGetTimeline(w http.ResponseWriter, r *http.Request) {
	campaignID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
		return
	}

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

	events, err := s.store.GetCampaignTimeline(r.Context(), campaignID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get timeline")
		return
	}

	if events == nil {
		events = []storage.TimelineEvent{}
	}

	writeJSON(w, http.StatusOK, events)
}

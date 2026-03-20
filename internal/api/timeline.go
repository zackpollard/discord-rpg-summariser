package api

import (
	"net/http"

	"discord-rpg-summariser/internal/storage"
)

func (s *Server) handleGetTimeline(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	limit, offset := parsePagination(r, 50)

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

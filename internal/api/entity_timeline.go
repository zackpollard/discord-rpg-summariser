package api

import (
	"net/http"

	"discord-rpg-summariser/internal/storage"
)

func (s *Server) handleGetEntityTimeline(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	entries, err := s.store.GetEntityTimeline(r.Context(), campaignID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get entity timeline")
		return
	}

	if entries == nil {
		entries = []storage.EntityTimelineEntry{}
	}

	writeJSON(w, http.StatusOK, entries)
}

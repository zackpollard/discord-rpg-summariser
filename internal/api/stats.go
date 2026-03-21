package api

import (
	"log"
	"net/http"
)

func (s *Server) handleGetCampaignStats(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	stats, err := s.store.GetCampaignStats(r.Context(), campaignID, s.guildID)
	if err != nil {
		log.Printf("GetCampaignStats error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get campaign stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

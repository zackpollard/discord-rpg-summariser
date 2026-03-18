package api

import (
	"net/http"
)

type statusResponse struct {
	Recording     bool             `json:"recording"`
	ActiveSession *sessionResponse `json:"active_session"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	guildID := r.URL.Query().Get("guild_id")
	if guildID == "" {
		guildID = s.guildID
	}

	sess, err := s.store.GetActiveSession(r.Context(), guildID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get status")
		return
	}

	resp := statusResponse{
		Recording:     sess != nil,
		ActiveSession: nil,
	}

	if sess != nil {
		sr := toSessionResponse(sess)
		resp.ActiveSession = &sr
	}

	writeJSON(w, http.StatusOK, resp)
}

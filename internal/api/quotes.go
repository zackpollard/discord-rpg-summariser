package api

import (
	"net/http"
	"time"
)

type quoteResponse struct {
	ID        int64     `json:"id"`
	SessionID int64     `json:"session_id"`
	Speaker   string    `json:"speaker"`
	Text      string    `json:"text"`
	StartTime float64   `json:"start_time"`
	Tone      *string   `json:"tone"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) handleGetSessionQuotes(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	quotes, err := s.store.GetQuotes(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get quotes")
		return
	}

	resp := make([]quoteResponse, len(quotes))
	for i := range quotes {
		q := &quotes[i]
		resp[i] = quoteResponse{
			ID:        q.ID,
			SessionID: q.SessionID,
			Speaker:   q.Speaker,
			Text:      q.Text,
			StartTime: q.StartTime,
			Tone:      q.Tone,
			CreatedAt: q.CreatedAt,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

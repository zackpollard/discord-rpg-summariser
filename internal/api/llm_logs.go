package api

import (
	"net/http"
	"time"
)

type llmLogResponse struct {
	ID         int64     `json:"id"`
	SessionID  *int64    `json:"session_id"`
	Operation  string    `json:"operation"`
	Prompt     string    `json:"prompt"`
	Response   string    `json:"response"`
	Error      *string   `json:"error"`
	DurationMS int       `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Server) handleGetLLMLogs(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	logs, err := s.store.GetLLMLogsForSession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get LLM logs")
		return
	}

	resp := make([]llmLogResponse, len(logs))
	for i, l := range logs {
		resp[i] = llmLogResponse{
			ID:         l.ID,
			SessionID:  l.SessionID,
			Operation:  l.Operation,
			Prompt:     l.Prompt,
			Response:   l.Response,
			Error:      l.Error,
			DurationMS: l.DurationMS,
			CreatedAt:  l.CreatedAt,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

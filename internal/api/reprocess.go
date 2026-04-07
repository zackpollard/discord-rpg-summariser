package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// SessionReprocessor re-runs the summarisation and extraction pipeline on an
// existing session's transcript data.
type SessionReprocessor interface {
	ReprocessSession(ctx context.Context, sessionID int64, retranscribe bool) error
	RerunStages(ctx context.Context, sessionID int64, stages []string) error
}

func (s *Server) SetSessionReprocessor(rp SessionReprocessor) {
	s.reprocessor = rp
}

type reprocessRequest struct {
	Retranscribe bool     `json:"retranscribe"`
	Stages       []string `json:"stages"` // optional: specific stages to re-run
}

func (s *Server) handleReprocessSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	sess, err := s.store.GetSession(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get session")
		return
	}

	if s.reprocessor == nil {
		writeError(w, http.StatusServiceUnavailable, "reprocessing not available")
		return
	}

	var req reprocessRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req) // ignore errors, defaults to false
	}

	if len(req.Stages) > 0 {
		go s.reprocessor.RerunStages(context.Background(), sess.ID, req.Stages)
	} else {
		go s.reprocessor.ReprocessSession(context.Background(), sess.ID, req.Retranscribe)
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "reprocessing started",
	})
}

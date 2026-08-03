package api

import (
	"context"
	"encoding/json"
	"log"
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

	// Only one reprocess per session at a time — concurrent runs fight over
	// the pipeline's shared progress state and interleave their DB writes.
	if !s.beginReprocess(sess.ID) {
		writeError(w, http.StatusConflict, "session is already being reprocessed")
		return
	}

	stages, retranscribe := req.Stages, req.Retranscribe
	go func() {
		defer s.endReprocess(sess.ID)
		// The goroutine is detached from the request, so a panic here would
		// take down the whole process rather than a single connection.
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("reprocess session %d: panic: %v", sess.ID, rec)
			}
		}()

		var err error
		if len(stages) > 0 {
			err = s.reprocessor.RerunStages(context.Background(), sess.ID, stages)
		} else {
			err = s.reprocessor.ReprocessSession(context.Background(), sess.ID, retranscribe)
		}
		if err != nil {
			log.Printf("reprocess session %d: %v", sess.ID, err)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "reprocessing started",
	})
}

// beginReprocess marks a session as reprocessing. It returns false when a
// reprocess for that session is already in flight.
func (s *Server) beginReprocess(sessionID int64) bool {
	s.reprocessMu.Lock()
	defer s.reprocessMu.Unlock()
	if _, ok := s.reprocessing[sessionID]; ok {
		return false
	}
	if s.reprocessing == nil {
		s.reprocessing = make(map[int64]struct{})
	}
	s.reprocessing[sessionID] = struct{}{}
	return true
}

func (s *Server) endReprocess(sessionID int64) {
	s.reprocessMu.Lock()
	delete(s.reprocessing, sessionID)
	s.reprocessMu.Unlock()
}

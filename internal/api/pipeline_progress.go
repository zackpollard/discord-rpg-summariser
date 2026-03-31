package api

import (
	"fmt"
	"net/http"
	"time"

	"discord-rpg-summariser/internal/bot"
)

// PipelineProgressProvider supplies pipeline progress for a session.
type PipelineProgressProvider interface {
	PipelineProgressFor(sessionID int64) *bot.PipelineProgress
}

func (s *Server) handlePipelineProgress(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	if s.progressP == nil {
		// No provider — send idle and close.
		fmt.Fprintf(w, "event: idle\ndata: {}\n\n")
		flusher.Flush()
		return
	}

	// Poll for an active progress tracker (the pipeline may not have started yet).
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var progress *bot.PipelineProgress
	timeout := time.After(30 * time.Second)
	for progress == nil {
		select {
		case <-r.Context().Done():
			return
		case <-timeout:
			// No pipeline started within timeout — session probably isn't processing.
			fmt.Fprintf(w, "event: idle\ndata: {}\n\n")
			flusher.Flush()
			return
		case <-ticker.C:
			progress = s.progressP.PipelineProgressFor(id)
		}
	}

	events, unsub := progress.Subscribe()
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			data, _ := evt.MarshalEvent()
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data)
			flusher.Flush()

			if evt.Type == "complete" {
				return
			}
		}
	}
}

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"discord-rpg-summariser/internal/voice"
)

// LiveTranscriptProvider supplies the current live transcription worker.
type LiveTranscriptProvider interface {
	LiveTranscriptWorker() *voice.LiveWorker
}

func (s *Server) handleLiveTranscript(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	if s.liveTP == nil {
		<-r.Context().Done()
		return
	}

	// Poll until a worker becomes available (session starts) or client disconnects.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var worker *voice.LiveWorker
	for worker == nil {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			worker = s.liveTP.LiveTranscriptWorker()
		}
	}

	events, unsub := worker.Subscribe()
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

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

	worker := s.liveTP.LiveTranscriptWorker()
	if worker == nil {
		// No active session; wait until client disconnects, they can reconnect later
		<-r.Context().Done()
		return
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

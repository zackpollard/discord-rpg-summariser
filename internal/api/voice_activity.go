package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleVoiceActivity streams live voice activity as Server-Sent Events.
// Each event is a JSON array of UserActivity objects sent every 250ms.
func (s *Server) handleVoiceActivity(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			var data []byte
			if s.voiceAP != nil {
				activity := s.voiceAP.VoiceActivity()
				if activity == nil {
					data = []byte("[]")
				} else {
					data, _ = json.Marshal(activity)
				}
			} else {
				data = []byte("[]")
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

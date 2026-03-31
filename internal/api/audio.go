package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"discord-rpg-summariser/internal/audio"

	"github.com/jackc/pgx/v5"
)

func (s *Server) handleGetSessionAudio(w http.ResponseWriter, r *http.Request) {
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

	if sess.AudioDir == "" {
		writeError(w, http.StatusNotFound, "no audio directory for session")
		return
	}

	mixedPath := filepath.Join(sess.AudioDir, "mixed.wav")

	if _, err := os.Stat(mixedPath); os.IsNotExist(err) {
		// Don't generate the mix on-the-fly for in-progress sessions —
		// the audio files are still being written. The pipeline generates
		// mixed.wav automatically once recording finishes.
		switch sess.Status {
		case "recording", "transcribing", "summarising":
			writeError(w, http.StatusConflict, "session is still in progress")
			return
		}

		// For older sessions that finished before auto-mixing was added,
		// generate on demand.
		if err := audio.MixFromDir(sess.AudioDir, mixedPath); err != nil {
			log.Printf("mix audio for session %d: %v", id, err)
			writeError(w, http.StatusInternalServerError, "failed to mix audio")
			return
		}
	}

	// Override the Content-Type that the CORS middleware sets for /api/ routes.
	w.Header().Set("Content-Type", "audio/wav")
	http.ServeFile(w, r, mixedPath)
}

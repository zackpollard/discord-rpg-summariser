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

	// Check if the mixed file already exists (cached).
	if _, err := os.Stat(mixedPath); os.IsNotExist(err) {
		// Find per-user WAV files in the audio directory.
		entries, err := os.ReadDir(sess.AudioDir)
		if err != nil {
			log.Printf("read audio dir %s: %v", sess.AudioDir, err)
			writeError(w, http.StatusInternalServerError, "failed to read audio directory")
			return
		}

		userFiles := make(map[string]string)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := filepath.Ext(name)
			if ext != ".wav" {
				continue
			}
			userID := name[:len(name)-len(ext)]
			if userID == "mixed" {
				continue
			}
			userFiles[userID] = filepath.Join(sess.AudioDir, name)
		}

		if len(userFiles) == 0 {
			writeError(w, http.StatusNotFound, "no audio files found")
			return
		}

		if err := audio.MixAndNormalize(userFiles, mixedPath); err != nil {
			log.Printf("mix audio for session %d: %v", id, err)
			writeError(w, http.StatusInternalServerError, "failed to mix audio")
			return
		}
	}

	// Override the Content-Type that the CORS middleware sets for /api/ routes.
	w.Header().Set("Content-Type", "audio/wav")
	http.ServeFile(w, r, mixedPath)
}

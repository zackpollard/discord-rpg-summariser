package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

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

	mixedPath, status, msg := ensureMixedAudio(sess.AudioDir, sess.Status)
	if status != 0 {
		writeError(w, status, msg)
		return
	}

	// Override the Content-Type that the CORS middleware sets for /api/ routes.
	w.Header().Set("Content-Type", "audio/wav")
	http.ServeFile(w, r, mixedPath)
}

// mixLocks serialises mixed.wav generation per audio directory, so two
// concurrent requests can't write the same file at the same time.
var (
	mixLocksMu sync.Mutex
	mixLocks   = make(map[string]*sync.Mutex)
)

func mixLock(mixedPath string) *sync.Mutex {
	mixLocksMu.Lock()
	defer mixLocksMu.Unlock()
	m, ok := mixLocks[mixedPath]
	if !ok {
		m = &sync.Mutex{}
		mixLocks[mixedPath] = m
	}
	return m
}

// ensureMixedAudio returns the path to a session's mixed.wav, generating it on
// demand for older sessions that finished before auto-mixing was added. When
// the mix can't be produced it returns the HTTP status and message to send;
// status is 0 on success.
func ensureMixedAudio(audioDir, sessionStatus string) (string, int, string) {
	mixedPath := filepath.Join(audioDir, "mixed.wav")
	if _, err := os.Stat(mixedPath); err == nil {
		return mixedPath, 0, ""
	}

	lock := mixLock(mixedPath)
	lock.Lock()
	defer lock.Unlock()

	// Another request may have generated it while we waited for the lock.
	if _, err := os.Stat(mixedPath); err == nil {
		return mixedPath, 0, ""
	}

	// Don't generate the mix on-the-fly for in-progress sessions —
	// the audio files are still being written. The pipeline generates
	// mixed.wav automatically once recording finishes.
	switch sessionStatus {
	case "recording", "transcribing", "summarising":
		return "", http.StatusConflict, "session is still in progress"
	}

	// Check that there are user WAVs to mix before attempting — sessions
	// that failed before recording anything have an empty audio dir.
	if !hasUserWAVs(audioDir) {
		return "", http.StatusNotFound, "session has no recorded audio"
	}

	if err := audio.MixFromDir(audioDir, mixedPath); err != nil {
		log.Printf("mix audio in %s: %v", audioDir, err)
		return "", http.StatusInternalServerError, "failed to mix audio"
	}

	return mixedPath, 0, ""
}

// hasUserWAVs returns true if the audio dir contains at least one per-user
// WAV file (excluding the mixed.wav output file).
func hasUserWAVs(audioDir string) bool {
	entries, err := os.ReadDir(audioDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) == ".wav" && name != "mixed.wav" {
			return true
		}
	}
	return false
}

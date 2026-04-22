package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"discord-rpg-summariser/internal/audio"

	"github.com/jackc/pgx/v5"
)

type userOffset struct {
	UserID         string  `json:"user_id"`
	DisplayName    string  `json:"display_name"`
	CharacterName  string  `json:"character_name,omitempty"`
	AutoOffset     float64 `json:"auto_offset"`
	OverrideOffset float64 `json:"override_offset"`
	HasOverride    bool    `json:"has_override"`
	DurationSec    float64 `json:"duration_sec"`
}

type syncResponse struct {
	SessionID int64        `json:"session_id"`
	Users     []userOffset `json:"users"`
}

// handleGetSessionSync returns per-user offsets (auto + override) for the
// sync correction UI.
func (s *Server) handleGetSessionSync(w http.ResponseWriter, r *http.Request) {
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

	autoOffsets := loadOffsetsFile(filepath.Join(sess.AudioDir, "offsets.json"))
	overrides := loadOffsetsFile(filepath.Join(sess.AudioDir, "offsets_override.json"))

	// Enumerate per-user WAVs.
	entries, err := os.ReadDir(sess.AudioDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read audio dir")
		return
	}

	// Load character mappings for nicer labels.
	charMap := make(map[string]string)
	if mappings, err := s.store.GetCharacterMappings(r.Context(), sess.CampaignID); err == nil {
		for _, m := range mappings {
			charMap[m.UserID] = m.CharacterName
		}
	}
	resolveDisplay := s.displayNameResolver()

	users := make([]userOffset, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".wav") {
			continue
		}
		userID := strings.TrimSuffix(name, ".wav")
		if userID == "mixed" {
			continue
		}

		u := userOffset{
			UserID:      userID,
			DisplayName: resolveDisplay(userID),
		}
		if cn, ok := charMap[userID]; ok {
			u.CharacterName = cn
		}
		if v, ok := autoOffsets[userID]; ok {
			u.AutoOffset = v
		}
		if v, ok := overrides[userID]; ok {
			u.OverrideOffset = v
			u.HasOverride = true
		} else {
			u.OverrideOffset = u.AutoOffset
		}
		// Best-effort duration calculation.
		if info, err := os.Stat(filepath.Join(sess.AudioDir, name)); err == nil && info.Size() > 44 {
			u.DurationSec = float64(info.Size()-44) / (48000 * 2)
		}
		users = append(users, u)
	}

	writeJSON(w, http.StatusOK, syncResponse{SessionID: id, Users: users})
}

// handleSetSessionSync writes an offsets_override.json file and deletes the
// cached mixed.wav so it gets regenerated on next request.
func (s *Server) handleSetSessionSync(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Overrides map[string]float64 `json:"overrides"`
		Clear     bool               `json:"clear"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	overridePath := filepath.Join(sess.AudioDir, "offsets_override.json")
	if req.Clear {
		_ = os.Remove(overridePath)
	} else {
		data, err := json.MarshalIndent(req.Overrides, "", "  ")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "marshal")
			return
		}
		if err := os.WriteFile(overridePath, data, 0o644); err != nil {
			log.Printf("write offsets_override.json for session %d: %v", id, err)
			writeError(w, http.StatusInternalServerError, "write override")
			return
		}
	}

	// Invalidate the cached mix — next mixed.wav request will regenerate
	// using the new offsets.
	_ = os.Remove(filepath.Join(sess.AudioDir, "mixed.wav"))

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleRemixSession forces regeneration of mixed.wav using current offsets
// (including overrides). Synchronous — returns after mixing finishes.
func (s *Server) handleRemixSession(w http.ResponseWriter, r *http.Request) {
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
	_ = os.Remove(mixedPath)
	if err := audio.MixFromDir(sess.AudioDir, mixedPath); err != nil {
		log.Printf("remix session %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "mix failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleGetUserAudio serves a single user's raw WAV file, for use in the
// sync correction UI where we play each track independently.
func (s *Server) handleGetUserAudio(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	userID := r.URL.Query().Get("user")
	if userID == "" || !isValidUserIDSegment(userID) {
		writeError(w, http.StatusBadRequest, "user query parameter required")
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
	path := filepath.Join(sess.AudioDir, userID+".wav")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "user audio not found")
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	http.ServeFile(w, r, path)
}

// handleGetUserWaveform returns peaks for a single user's WAV.
func (s *Server) handleGetUserWaveform(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	userID := r.URL.Query().Get("user")
	if userID == "" || !isValidUserIDSegment(userID) {
		writeError(w, http.StatusBadRequest, "user query parameter required")
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
	path := filepath.Join(sess.AudioDir, userID+".wav")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "user audio not found")
		return
	}

	startSec, endSec, parseErr := parseWaveformRange(r)
	if parseErr != nil {
		writeError(w, http.StatusBadRequest, parseErr.Error())
		return
	}
	numPeaks := desiredWaveformPeaks(r, path, startSec, endSec)
	peaks, fullDuration, err := computePeaksRange(path, numPeaks, startSec, endSec)
	if err != nil {
		log.Printf("user waveform: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to compute waveform")
		return
	}
	w.Header().Set("Cache-Control", "max-age=3600")
	writeJSON(w, http.StatusOK, waveformResponse{
		Peaks:        peaks,
		StartSec:     startSec,
		EndSec:       endSec,
		FullDuration: fullDuration,
	})
}

func loadOffsetsFile(path string) map[string]float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m map[string]float64
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

// isValidUserIDSegment guards against path traversal via the user query
// param. Discord IDs are numeric snowflakes; reject anything else.
func isValidUserIDSegment(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}


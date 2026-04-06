package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"discord-rpg-summariser/internal/audio"
	"discord-rpg-summariser/internal/storage"

	"github.com/jackc/pgx/v5"
)

// SoundboardPlayer plays audio through the bot's active voice connection.
type SoundboardPlayer interface {
	PlayClipInVoice(wavPath string) error
}

type createClipRequest struct {
	SessionID int64    `json:"session_id"`
	Name      string   `json:"name"`
	StartTime float64  `json:"start_time"`
	EndTime   float64  `json:"end_time"`
	UserIDs   []string `json:"user_ids"`
}

func (s *Server) handleCreateClip(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	var req createClipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.EndTime <= req.StartTime {
		writeError(w, http.StatusBadRequest, "end_time must be after start_time")
		return
	}
	if len(req.UserIDs) == 0 {
		writeError(w, http.StatusBadRequest, "at least one user_id is required")
		return
	}

	sess, err := s.store.GetSession(r.Context(), req.SessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if sess.AudioDir == "" {
		writeError(w, http.StatusNotFound, "no audio directory for session")
		return
	}

	// Build the user files map for selected users.
	userFiles := make(map[string]string)
	for _, uid := range req.UserIDs {
		wavPath := filepath.Join(sess.AudioDir, uid+".wav")
		if _, err := os.Stat(wavPath); err == nil {
			userFiles[uid] = wavPath
		}
	}
	if len(userFiles) == 0 {
		writeError(w, http.StatusBadRequest, "no audio files found for selected users")
		return
	}

	joinOffsets := audio.LoadJoinOffsets(sess.AudioDir)

	// Create output directory and mix the clip.
	clipDir := filepath.Join("data", "soundboard", fmt.Sprintf("%d", campaignID))
	os.MkdirAll(clipDir, 0o755)
	filename := fmt.Sprintf("%d-%s.wav", time.Now().Unix(), sanitizeClipName(req.Name))
	audioPath := filepath.Join(clipDir, filename)

	if err := audio.MixClip(userFiles, audioPath, joinOffsets, req.StartTime, req.EndTime); err != nil {
		log.Printf("create clip: mix error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create clip: "+err.Error())
		return
	}

	sid := req.SessionID
	id, err := s.store.InsertSoundboardClip(r.Context(), storage.SoundboardClip{
		CampaignID: campaignID,
		SessionID:  &sid,
		Name:       req.Name,
		AudioPath:  audioPath,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		UserIDs:    req.UserIDs,
	})
	if err != nil {
		log.Printf("create clip: insert error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save clip")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) handleListClips(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	clips, err := s.store.ListSoundboardClips(r.Context(), campaignID)
	if err != nil {
		log.Printf("list clips: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list clips")
		return
	}

	type clipResponse struct {
		ID        int64    `json:"id"`
		SessionID *int64   `json:"session_id"`
		Name      string   `json:"name"`
		StartTime float64  `json:"start_time"`
		EndTime   float64  `json:"end_time"`
		UserIDs   []string `json:"user_ids"`
		CreatedAt string   `json:"created_at"`
	}

	resp := make([]clipResponse, len(clips))
	for i, c := range clips {
		resp[i] = clipResponse{
			ID:        c.ID,
			SessionID: c.SessionID,
			Name:      c.Name,
			StartTime: c.StartTime,
			EndTime:   c.EndTime,
			UserIDs:   c.UserIDs,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeleteClip(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "clipId")
	if !ok {
		return
	}

	clip, err := s.store.GetSoundboardClip(r.Context(), id)
	if err == pgx.ErrNoRows || clip == nil {
		writeError(w, http.StatusNotFound, "clip not found")
		return
	}

	os.Remove(clip.AudioPath)

	if err := s.store.DeleteSoundboardClip(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete clip")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetClipAudio(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "clipId")
	if !ok {
		return
	}

	clip, err := s.store.GetSoundboardClip(r.Context(), id)
	if err == pgx.ErrNoRows || clip == nil {
		writeError(w, http.StatusNotFound, "clip not found")
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	http.ServeFile(w, r, clip.AudioPath)
}

func (s *Server) handlePlayClip(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "clipId")
	if !ok {
		return
	}

	if s.soundboardP == nil {
		writeError(w, http.StatusServiceUnavailable, "soundboard not available")
		return
	}

	clip, err := s.store.GetSoundboardClip(r.Context(), id)
	if err == pgx.ErrNoRows || clip == nil {
		writeError(w, http.StatusNotFound, "clip not found")
		return
	}

	go func() {
		if err := s.soundboardP.PlayClipInVoice(clip.AudioPath); err != nil {
			log.Printf("soundboard play: %v", err)
		}
	}()

	writeJSON(w, http.StatusOK, map[string]string{"status": "playing"})
}

func sanitizeClipName(name string) string {
	var clean []byte
	for _, c := range []byte(name) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			clean = append(clean, c)
		}
	}
	if len(clean) == 0 {
		return "clip"
	}
	if len(clean) > 50 {
		clean = clean[:50]
	}
	return string(clean)
}

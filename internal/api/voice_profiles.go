package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func (s *Server) handleUploadVoiceProfile(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	// Parse multipart form (max 50MB).
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	transcript := r.FormValue("transcript")

	file, header, err := r.FormFile("audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, "audio file is required")
		return
	}
	defer file.Close()

	// Save the uploaded audio file.
	audioDir := filepath.Join("data", "voice-profiles", fmt.Sprintf("%d", campaignID))
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		log.Printf("create voice profile dir: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save audio")
		return
	}

	filename := fmt.Sprintf("%d-%s%s", time.Now().Unix(), sanitizeProfileFilename(name), filepath.Ext(header.Filename))
	audioPath := filepath.Join(audioDir, filename)

	dst, err := os.Create(audioPath)
	if err != nil {
		log.Printf("create voice profile file: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save audio")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		log.Printf("write voice profile file: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save audio")
		return
	}

	id, err := s.store.InsertVoiceProfile(r.Context(), campaignID, name, audioPath, transcript)
	if err != nil {
		log.Printf("insert voice profile: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save profile")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) handleListVoiceProfiles(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	profiles, err := s.store.GetVoiceProfiles(r.Context(), campaignID)
	if err != nil {
		log.Printf("list voice profiles: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list profiles")
		return
	}

	type profileResponse struct {
		ID         int64     `json:"id"`
		Name       string    `json:"name"`
		Transcript string    `json:"transcript"`
		CreatedAt  time.Time `json:"created_at"`
	}

	resp := make([]profileResponse, len(profiles))
	for i, p := range profiles {
		resp[i] = profileResponse{
			ID:         p.ID,
			Name:       p.Name,
			Transcript: p.Transcript,
			CreatedAt:  p.CreatedAt,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeleteVoiceProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "profileId")
	if !ok {
		return
	}

	profile, err := s.store.GetVoiceProfile(r.Context(), id)
	if err != nil || profile == nil {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}

	// Remove audio file.
	os.Remove(profile.AudioPath)

	if err := s.store.DeleteVoiceProfile(r.Context(), id); err != nil {
		log.Printf("delete voice profile: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete profile")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetVoiceProfileAudio serves the uploaded audio file for preview.
func (s *Server) handleGetVoiceProfileAudio(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("profileId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid profile ID")
		return
	}

	profile, err := s.store.GetVoiceProfile(r.Context(), id)
	if err != nil || profile == nil {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	http.ServeFile(w, r, profile.AudioPath)
}

func sanitizeProfileFilename(name string) string {
	var clean []byte
	for _, c := range []byte(name) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			clean = append(clean, c)
		}
	}
	if len(clean) == 0 {
		return "voice"
	}
	return string(clean)
}

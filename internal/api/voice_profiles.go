package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"discord-rpg-summariser/internal/transcribe"
)

// TranscriberProvider provides access to a shared transcriber instance.
type TranscriberProvider interface {
	AcquireTranscriber() (transcribe.Transcriber, error)
	ReleaseTranscriber()
}

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
	dst.Close()

	// Normalize to 48kHz mono 16-bit WAV (no trim yet — we'll find a
	// natural boundary after transcription).
	normalizedPath := audioPath + ".norm.wav"
	if err := normalizeAudio(audioPath, normalizedPath, 0); err != nil {
		log.Printf("voice profile normalize: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to process audio — ensure the file is a valid audio format")
		os.Remove(audioPath)
		return
	}
	os.Remove(audioPath)
	wavPath := strings.TrimSuffix(audioPath, filepath.Ext(audioPath)) + ".wav"
	if err := os.Rename(normalizedPath, wavPath); err != nil {
		log.Printf("voice profile rename: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save audio")
		return
	}
	audioPath = wavPath

	// Transcribe and find a natural trim point at a sentence boundary.
	if s.transcriberP != nil {
		segments, trimSec, err := s.transcribeAndTrim(r.Context(), audioPath, 10.0)
		if err != nil {
			log.Printf("voice profile transcribe/trim: %v", err)
		} else {
			// Build transcript from segments up to the trim point.
			var parts []string
			for _, seg := range segments {
				if seg.EndTime > trimSec {
					break
				}
				text := strings.TrimSpace(seg.Text)
				if text != "" {
					parts = append(parts, text)
				}
			}
			if len(parts) > 0 && transcript == "" {
				transcript = strings.Join(parts, " ")
				log.Printf("voice profile auto-transcribed: %q", transcript)
			}

			// Re-trim the WAV to the natural boundary.
			if trimSec > 0 && trimSec < 30 {
				trimmedPath := audioPath + ".trim.wav"
				if err := normalizeAudio(audioPath, trimmedPath, trimSec); err == nil {
					os.Remove(audioPath)
					os.Rename(trimmedPath, audioPath)
					log.Printf("voice profile trimmed to %.1fs (sentence boundary)", trimSec)
				}
			}
		}
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

// normalizeAudio converts any audio file to 48kHz mono 16-bit WAV using ffmpeg.
// If trimSec > 0, the output is trimmed to that duration.
func normalizeAudio(inputPath, outputPath string, trimSec float64) error {
	args := []string{"-y", "-i", inputPath}
	if trimSec > 0 {
		args = append(args, "-t", fmt.Sprintf("%.2f", trimSec))
	}
	args = append(args,
		"-ar", "48000",
		"-ac", "1",
		"-sample_fmt", "s16",
		"-f", "wav",
		outputPath,
	)
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, string(out))
	}
	return nil
}

// transcribeAndTrim transcribes a WAV file and finds a natural sentence
// boundary near maxDuration seconds. Returns the segments and the trim point.
func (s *Server) transcribeAndTrim(ctx context.Context, wavPath string, maxDuration float64) ([]transcribe.Segment, float64, error) {
	t, err := s.transcriberP.AcquireTranscriber()
	if err != nil {
		return nil, 0, fmt.Errorf("acquire transcriber: %w", err)
	}
	defer s.transcriberP.ReleaseTranscriber()

	segments, err := t.TranscribeFile(ctx, wavPath)
	if err != nil {
		return nil, 0, fmt.Errorf("transcribe: %w", err)
	}

	// Find the last segment that ends at or before maxDuration.
	// This gives us a natural sentence/phrase boundary.
	trimAt := maxDuration
	for _, seg := range segments {
		if seg.EndTime <= maxDuration {
			trimAt = seg.EndTime
		}
	}

	return segments, trimAt, nil
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

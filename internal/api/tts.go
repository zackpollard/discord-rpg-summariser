package api

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"discord-rpg-summariser/internal/audio"
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/tts"
)

type ttsService struct {
	synth *tts.Synthesizer
	store *storage.Store

	mu       sync.Mutex
	progress float64 // 0.0-1.0 during generation, -1 = idle
}

// NewTTSService creates a TTS service that synthesizes recaps.
func NewTTSService(synth *tts.Synthesizer, store *storage.Store) *ttsService {
	svc := &ttsService{synth: synth, store: store, progress: -1}
	synth.SetProgressCallback(func(p float64) {
		svc.mu.Lock()
		svc.progress = p
		svc.mu.Unlock()
	})
	return svc
}

func (t *ttsService) extractRef(campaignID int64, userID string) (*tts.ReferenceClip, error) {
	return tts.ExtractReference(t.store, campaignID, userID)
}

func (t *ttsService) extractProfileRef(ctx context.Context, profileID int64) (*tts.ReferenceClip, error) {
	profile, err := t.store.GetVoiceProfile(ctx, profileID)
	if err != nil || profile == nil {
		return nil, fmt.Errorf("voice profile %d not found", profileID)
	}

	samples, err := audio.LoadRaw48k(profile.AudioPath)
	if err != nil {
		return nil, fmt.Errorf("load profile audio: %w", err)
	}

	// Limit to first 10 seconds.
	maxSamples := 10 * 48000
	if len(samples) > maxSamples {
		samples = samples[:maxSamples]
	}

	return &tts.ReferenceClip{
		Samples:    samples,
		SampleRate: 48000,
		Text:       profile.Transcript,
	}, nil
}

func (t *ttsService) getProgress() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.progress
}

func (s *Server) handleListRecapVoices(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	userIDs, err := s.store.GetUsersWithAudio(r.Context(), campaignID)
	if err != nil {
		log.Printf("list recap voices: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list voices")
		return
	}

	type voiceEntry struct {
		UserID      string `json:"user_id"`
		ProfileID   int64  `json:"profile_id,omitempty"`
		DisplayName string `json:"display_name"`
		IsCustom    bool   `json:"is_custom"`
	}

	voices := make([]voiceEntry, 0, len(userIDs))

	// Add custom voice profiles first.
	profiles, _ := s.store.GetVoiceProfiles(r.Context(), campaignID)
	for _, p := range profiles {
		voices = append(voices, voiceEntry{
			ProfileID:   p.ID,
			DisplayName: p.Name,
			IsCustom:    true,
		})
	}

	// Add campaign members with audio.
	for _, uid := range userIDs {
		name := uid
		u, err := s.store.GetDiscordUser(r.Context(), uid, s.guildID)
		if err == nil {
			name = u.DisplayName
		}
		if cn, _ := s.store.GetCharacterName(r.Context(), uid, campaignID); cn != "" {
			name = cn
		}
		voices = append(voices, voiceEntry{UserID: uid, DisplayName: name})
	}

	writeJSON(w, http.StatusOK, voices)
}

func (s *Server) handleGetRecapTTS(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	voiceUserID := r.URL.Query().Get("voice")
	profileIDStr := r.URL.Query().Get("profile")
	if voiceUserID == "" && profileIDStr == "" {
		writeError(w, http.StatusBadRequest, "voice or profile query parameter required")
		return
	}

	if s.ttsSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "TTS not configured")
		return
	}

	source := r.URL.Query().Get("source") // "recap" (default) or "previously-on"

	campaign, err := s.store.GetCampaign(r.Context(), campaignID)
	if err != nil {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}

	// Determine the text to synthesize.
	var ttsText string
	if source == "previously-on" {
		// Generate previously-on text for TTS.
		if s.summariser == nil {
			writeError(w, http.StatusServiceUnavailable, "summariser not available")
			return
		}
		sessions, err := s.store.GetLatestCompleteSessions(r.Context(), campaignID, 1)
		if err != nil || len(sessions) == 0 || sessions[0].Summary == nil {
			writeError(w, http.StatusNotFound, "no sessions with summaries")
			return
		}
		result, err := s.summariser.GeneratePreviouslyOn(r.Context(), *sessions[0].Summary, campaign.Recap)
		if err != nil {
			log.Printf("recap tts: generate previously-on: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to generate previously-on text")
			return
		}
		ttsText = result.Text
	} else {
		if campaign.Recap == "" {
			writeError(w, http.StatusNotFound, "no recap generated yet")
			return
		}
		ttsText = campaign.Recap
	}

	// Extract reference audio — either from a custom profile or a campaign member.
	var ref *tts.ReferenceClip
	if profileIDStr != "" {
		profileID, _ := strconv.ParseInt(profileIDStr, 10, 64)
		ref, err = s.ttsSvc.extractProfileRef(r.Context(), profileID)
	} else {
		ref, err = s.ttsSvc.extractRef(campaignID, voiceUserID)
	}
	if err != nil {
		log.Printf("recap tts: extract ref: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to extract reference audio")
		return
	}

	// Mark as generating.
	s.ttsSvc.mu.Lock()
	s.ttsSvc.progress = 0
	s.ttsSvc.mu.Unlock()
	defer func() {
		s.ttsSvc.mu.Lock()
		s.ttsSvc.progress = -1
		s.ttsSvc.mu.Unlock()
	}()

	samples, sampleRate, err := s.ttsSvc.synth.Synthesize(
		ttsText, ref.Samples, ref.SampleRate, ref.Text,
	)
	if err != nil {
		log.Printf("recap tts: synthesize: %v", err)
		writeError(w, http.StatusInternalServerError, "TTS generation failed")
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "no-store")
	writeWAVResponse(w, samples, sampleRate)
}

// handleTTSProgress is an SSE endpoint that streams TTS generation progress.
func (s *Server) handleTTSProgress(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	if s.ttsSvc == nil {
		fmt.Fprintf(w, "data: {\"progress\": -1}\n\n")
		flusher.Flush()
		return
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			p := s.ttsSvc.getProgress()
			fmt.Fprintf(w, "data: {\"progress\": %.4f}\n\n", p)
			flusher.Flush()

			if p < 0 || p >= 1.0 {
				return
			}
		}
	}
}

// handleGetRefAudio serves the raw reference audio clip for debugging.
func (s *Server) handleGetRefAudio(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	voiceUserID := r.URL.Query().Get("voice")
	if voiceUserID == "" {
		writeError(w, http.StatusBadRequest, "voice query parameter required")
		return
	}
	if s.ttsSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "TTS not configured")
		return
	}

	ref, err := s.ttsSvc.extractRef(campaignID, voiceUserID)
	if err != nil {
		log.Printf("ref audio: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to extract reference audio")
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "no-store")
	writeWAVResponse(w, ref.Samples, ref.SampleRate)
}

func writeWAVResponse(w http.ResponseWriter, samples []float32, sampleRate int) {
	const (
		numChannels = 1
		bitsPerSamp = 16
	)
	dataSize := uint32(len(samples) * 2)
	byteRate := uint32(sampleRate * numChannels * (bitsPerSamp / 8))
	blockAlign := uint16(numChannels * (bitsPerSamp / 8))
	fileSize := 36 + dataSize

	var hdr [44]byte
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], fileSize)
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)
	binary.LittleEndian.PutUint16(hdr[20:22], 1)
	binary.LittleEndian.PutUint16(hdr[22:24], numChannels)
	binary.LittleEndian.PutUint32(hdr[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(hdr[28:32], byteRate)
	binary.LittleEndian.PutUint16(hdr[32:34], blockAlign)
	binary.LittleEndian.PutUint16(hdr[34:36], bitsPerSamp)
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], dataSize)
	w.Write(hdr[:])

	buf := make([]byte, 2)
	for _, s := range samples {
		if s > 1.0 {
			s = 1.0
		} else if s < -1.0 {
			s = -1.0
		}
		binary.LittleEndian.PutUint16(buf, uint16(int16(s*32767.0)))
		w.Write(buf)
	}
}

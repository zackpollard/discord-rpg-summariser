package api

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	progress float64            // 0.0-1.0 during generation, -1 = idle
	source   string             // active generation source ("recap" or "previously-on")
	voiceKey string             // active generation voice key ("user:123" or "profile:45")
	cancel   context.CancelFunc // cancels the active generation
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

type ttsProgressState struct {
	Progress float64 `json:"progress"`
	Source   string  `json:"source,omitempty"`
	VoiceKey string  `json:"voice_key,omitempty"`
}

func (t *ttsService) getProgressState() ttsProgressState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return ttsProgressState{
		Progress: t.progress,
		Source:   t.source,
		VoiceKey: t.voiceKey,
	}
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
	if source == "" {
		source = "recap"
	}
	regenerate := r.URL.Query().Get("regenerate") == "true"

	// Build voice key for cache lookup.
	var voiceKey string
	if profileIDStr != "" {
		voiceKey = "profile:" + profileIDStr
	} else {
		voiceKey = storage.VoiceKeyForUser(voiceUserID)
	}

	// Check cache unless regeneration is requested.
	if !regenerate {
		cached, err := s.store.GetTTSCache(r.Context(), campaignID, source, voiceKey)
		if err == nil && cached != nil {
			if _, statErr := os.Stat(cached.AudioPath); statErr == nil {
				http.ServeFile(w, r, cached.AudioPath)
				return
			}
		}
	}

	campaign, err := s.store.GetCampaign(r.Context(), campaignID)
	if err != nil {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}

	// Determine the text to synthesize.
	var ttsText string
	if source == "previously-on" {
		ttsText = campaign.PreviouslyOn
		if ttsText == "" {
			writeError(w, http.StatusNotFound, "no previously-on text generated yet — generate it from the recap page first")
			return
		}
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

	// Mark as generating with a cancellable context.
	genCtx, genCancel := context.WithCancel(r.Context())
	s.ttsSvc.mu.Lock()
	s.ttsSvc.progress = 0
	s.ttsSvc.source = source
	s.ttsSvc.voiceKey = voiceKey
	s.ttsSvc.cancel = genCancel
	s.ttsSvc.mu.Unlock()
	defer func() {
		s.ttsSvc.mu.Lock()
		s.ttsSvc.progress = -1
		s.ttsSvc.source = ""
		s.ttsSvc.voiceKey = ""
		s.ttsSvc.cancel = nil
		s.ttsSvc.mu.Unlock()
		genCancel()
	}()

	samples, sampleRate, err := s.ttsSvc.synth.Synthesize(
		genCtx, ttsText, ref.Samples, ref.SampleRate, ref.Text,
	)
	if err != nil {
		log.Printf("recap tts: synthesize: %v", err)
		writeError(w, http.StatusInternalServerError, "TTS generation failed")
		return
	}

	// Save to cache.
	cacheDir := filepath.Join("data", "tts-cache")
	os.MkdirAll(cacheDir, 0o755)
	wavPath := filepath.Join(cacheDir, fmt.Sprintf("%s-%d-%s.wav", source, campaignID, sanitizeVoiceKey(voiceKey)))

	if err := tts.WriteWAV(wavPath, samples, sampleRate); err != nil {
		log.Printf("recap tts: save cache: %v", err)
	} else {
		_ = s.store.UpsertTTSCache(r.Context(), storage.TTSAudioCache{
			CampaignID: campaignID,
			Source:     source,
			VoiceKey:   voiceKey,
			AudioPath:  wavPath,
		})
	}

	w.Header().Set("Content-Type", "audio/wav")
	writeWAVResponse(w, samples, sampleRate)
}

func sanitizeVoiceKey(key string) string {
	return strings.ReplaceAll(key, ":", "-")
}

func (s *Server) handleListCachedTTS(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	entries, err := s.store.ListTTSCache(r.Context(), campaignID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list cached TTS")
		return
	}

	type cacheEntry struct {
		Source      string `json:"source"`
		VoiceKey    string `json:"voice_key"`
		GeneratedAt string `json:"generated_at"`
	}
	result := make([]cacheEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, cacheEntry{
			Source:      e.Source,
			VoiceKey:    e.VoiceKey,
			GeneratedAt: e.GeneratedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"entries": result})
}

func (s *Server) handleCancelTTS(w http.ResponseWriter, r *http.Request) {
	if s.ttsSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "TTS not configured")
		return
	}

	s.ttsSvc.mu.Lock()
	cancel := s.ttsSvc.cancel
	s.ttsSvc.mu.Unlock()

	if cancel == nil {
		writeError(w, http.StatusNotFound, "no generation in progress")
		return
	}

	cancel()
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
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

	enc := json.NewEncoder(w)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			state := s.ttsSvc.getProgressState()
			fmt.Fprintf(w, "data: ")
			enc.Encode(state)
			fmt.Fprintf(w, "\n")
			flusher.Flush()

			if state.Progress < 0 || state.Progress >= 1.0 {
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

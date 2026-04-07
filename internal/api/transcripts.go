package api

import (
	"net/http"
	"time"
)

type transcriptSegmentResponse struct {
	ID             int64     `json:"id"`
	SessionID      int64     `json:"session_id"`
	UserID         string    `json:"user_id"`
	DisplayName    string    `json:"display_name"`
	CharacterName  *string   `json:"character_name"`
	StartTime      float64   `json:"start_time"`
	EndTime        float64   `json:"end_time"`
	Text           string    `json:"text"`
	CorrectedText  *string   `json:"corrected_text,omitempty"`
	Classification *string   `json:"classification,omitempty"`
	Scene          *string   `json:"scene,omitempty"`
	NPCVoice       *string   `json:"npc_voice,omitempty"`
	Tone           *string   `json:"tone,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Server) handleGetTranscript(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	session, err := s.store.GetSession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get session")
		return
	}

	segments, err := s.store.GetTranscript(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get transcript")
		return
	}

	// Resolve character names from current mappings using session's campaign.
	charMappings, _ := s.store.GetCharacterMappings(r.Context(), session.CampaignID)
	charMap := make(map[string]string, len(charMappings))
	for _, m := range charMappings {
		charMap[m.UserID] = m.CharacterName
	}

	// Label the DM if they don't have a character mapping.
	campaign, _ := s.store.GetCampaign(r.Context(), session.CampaignID)
	if campaign != nil && campaign.DMUserID != nil {
		if _, hasMapped := charMap[*campaign.DMUserID]; !hasMapped {
			charMap[*campaign.DMUserID] = "DM"
		}
	}

	// Load annotations if available.
	annotations, _ := s.store.GetAnnotations(r.Context(), id)
	annMap := make(map[int64]int, len(annotations))
	for i, a := range annotations {
		annMap[a.SegmentID] = i
	}

	resolveDisplay := s.displayNameResolver()

	resp := make([]transcriptSegmentResponse, len(segments))
	for i := range segments {
		seg := &segments[i]

		var charName *string
		if name, ok := charMap[seg.UserID]; ok {
			charName = &name
		}

		resp[i] = transcriptSegmentResponse{
			ID:            seg.ID,
			SessionID:     seg.SessionID,
			UserID:        seg.UserID,
			DisplayName:   resolveDisplay(seg.UserID),
			CharacterName: charName,
			StartTime:     seg.StartTime,
			EndTime:       seg.EndTime,
			Text:          seg.Text,
			CreatedAt:     seg.CreatedAt,
		}

		// Attach annotation data if available.
		if idx, ok := annMap[seg.ID]; ok {
			a := &annotations[idx]
			resp[i].Classification = &a.Classification
			resp[i].CorrectedText = a.CorrectedText
			resp[i].Scene = a.Scene
			resp[i].NPCVoice = a.NPCVoice
			resp[i].Tone = a.Tone
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

package api

import (
	"net/http"
	"strconv"
	"time"
)

type transcriptSegmentResponse struct {
	ID            int64     `json:"id"`
	SessionID     int64     `json:"session_id"`
	UserID        string    `json:"user_id"`
	DisplayName   string    `json:"display_name"`
	CharacterName *string   `json:"character_name"`
	StartTime     float64   `json:"start_time"`
	EndTime       float64   `json:"end_time"`
	Text          string    `json:"text"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Server) handleGetTranscript(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
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

	// Resolve character names from current mappings (not stored values)
	guildID := s.guildID
	if session != nil {
		guildID = session.GuildID
	}
	charMappings, _ := s.store.GetCharacterMappings(r.Context(), guildID)
	charMap := make(map[string]string, len(charMappings))
	for _, m := range charMappings {
		charMap[m.UserID] = m.CharacterName
	}

	resp := make([]transcriptSegmentResponse, len(segments))
	for i := range segments {
		seg := &segments[i]

		displayName := seg.UserID
		if s.memberP != nil {
			displayName = s.memberP.ResolveUsername(seg.UserID)
		}

		var charName *string
		if name, ok := charMap[seg.UserID]; ok {
			charName = &name
		}

		resp[i] = transcriptSegmentResponse{
			ID:            seg.ID,
			SessionID:     seg.SessionID,
			UserID:        seg.UserID,
			DisplayName:   displayName,
			CharacterName: charName,
			StartTime:     seg.StartTime,
			EndTime:       seg.EndTime,
			Text:          seg.Text,
			CreatedAt:     seg.CreatedAt,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

package api

import (
	"net/http"
	"strconv"
	"time"

	"discord-rpg-summariser/internal/storage"
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

	segments, err := s.store.GetTranscript(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get transcript")
		return
	}

	resp := make([]transcriptSegmentResponse, len(segments))
	for i := range segments {
		seg := &segments[i]
		displayName := seg.UserID
		if s.memberP != nil {
			displayName = s.memberP.ResolveUsername(seg.UserID)
		}
		resp[i] = transcriptSegmentResponse{
			ID:            seg.ID,
			SessionID:     seg.SessionID,
			UserID:        seg.UserID,
			DisplayName:   displayName,
			CharacterName: seg.CharacterName,
			StartTime:     seg.StartTime,
			EndTime:       seg.EndTime,
			Text:          seg.Text,
			CreatedAt:     seg.CreatedAt,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

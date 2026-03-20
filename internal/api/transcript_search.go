package api

import (
	"net/http"
	"strconv"
	"time"
)

type transcriptSearchResultResponse struct {
	SegmentID     int64     `json:"segment_id"`
	SessionID     int64     `json:"session_id"`
	UserID        string    `json:"user_id"`
	DisplayName   string    `json:"display_name"`
	CharacterName *string   `json:"character_name"`
	StartTime     float64   `json:"start_time"`
	EndTime       float64   `json:"end_time"`
	Text          string    `json:"text"`
	Headline      string    `json:"headline"`
	SessionAt     time.Time `json:"session_started_at"`
}

type transcriptSearchResponse struct {
	Results []transcriptSearchResultResponse `json:"results"`
	Total   int                              `json:"total"`
	Limit   int                              `json:"limit"`
	Offset  int                              `json:"offset"`
}

func (s *Server) handleTranscriptSearch(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter q is required")
		return
	}

	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	results, total, err := s.store.SearchTranscripts(r.Context(), campaignID, query, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search transcripts")
		return
	}

	// Resolve character names for the campaign.
	charMappings, _ := s.store.GetCharacterMappings(r.Context(), campaignID)
	charMap := make(map[string]string, len(charMappings))
	for _, m := range charMappings {
		charMap[m.UserID] = m.CharacterName
	}

	// Resolve display names once per unique user.
	nameCache := make(map[string]string)
	resolveDisplay := func(userID string) string {
		if name, ok := nameCache[userID]; ok {
			return name
		}
		name := userID
		if s.memberP != nil {
			name = s.memberP.ResolveUsername(userID)
		}
		nameCache[userID] = name
		return name
	}

	resp := make([]transcriptSearchResultResponse, len(results))
	for i := range results {
		res := &results[i]

		var charName *string
		if name, ok := charMap[res.UserID]; ok {
			charName = &name
		}

		resp[i] = transcriptSearchResultResponse{
			SegmentID:     res.SegmentID,
			SessionID:     res.SessionID,
			UserID:        res.UserID,
			DisplayName:   resolveDisplay(res.UserID),
			CharacterName: charName,
			StartTime:     res.StartTime,
			EndTime:       res.EndTime,
			Text:          res.Text,
			Headline:      res.Headline,
			SessionAt:     res.SessionAt,
		}
	}

	writeJSON(w, http.StatusOK, transcriptSearchResponse{
		Results: resp,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	})
}

package api

import (
	"net/http"
	"strconv"
	"time"

	"discord-rpg-summariser/internal/storage"

	"github.com/jackc/pgx/v5"
)

type sessionResponse struct {
	ID         int64      `json:"id"`
	GuildID    string     `json:"guild_id"`
	CampaignID int64      `json:"campaign_id"`
	ChannelID  string     `json:"channel_id"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at"`
	Status     string     `json:"status"`
	Summary    *string    `json:"summary"`
	KeyEvents  []string   `json:"key_events"`
	Title      *string    `json:"title"`
	CreatedAt  time.Time  `json:"created_at"`
}

func toSessionResponse(sess *storage.Session) sessionResponse {
	events := sess.KeyEvents
	if events == nil {
		events = []string{}
	}
	return sessionResponse{
		ID:         sess.ID,
		GuildID:    sess.GuildID,
		CampaignID: sess.CampaignID,
		ChannelID:  sess.ChannelID,
		StartedAt:  sess.StartedAt,
		EndedAt:    sess.EndedAt,
		Status:     sess.Status,
		Summary:    sess.Summary,
		KeyEvents:  events,
		Title:      sess.Title,
		CreatedAt:  sess.CreatedAt,
	}
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r, 20)

	guildID := r.URL.Query().Get("guild_id")
	if guildID == "" {
		guildID = s.guildID
	}

	var campaignID int64
	if v := r.URL.Query().Get("campaign_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			campaignID = n
		}
	}

	sessions, err := s.store.ListSessions(r.Context(), guildID, campaignID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	resp := make([]sessionResponse, len(sessions))
	for i := range sessions {
		resp[i] = toSessionResponse(&sessions[i])
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, toSessionResponse(sess))
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	if err := s.store.DeleteSession(r.Context(), id); err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete session")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

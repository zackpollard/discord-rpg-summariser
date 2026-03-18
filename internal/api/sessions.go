package api

import (
	"net/http"
	"strconv"
	"time"

	"discord-rpg-summariser/internal/storage"

	"github.com/jackc/pgx/v5"
)

type sessionResponse struct {
	ID        int64      `json:"id"`
	GuildID   string     `json:"guild_id"`
	ChannelID string     `json:"channel_id"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
	Status    string     `json:"status"`
	Summary   *string    `json:"summary"`
	KeyEvents []string   `json:"key_events"`
	CreatedAt time.Time  `json:"created_at"`
}

func toSessionResponse(sess *storage.Session) sessionResponse {
	events := sess.KeyEvents
	if events == nil {
		events = []string{}
	}
	return sessionResponse{
		ID:        sess.ID,
		GuildID:   sess.GuildID,
		ChannelID: sess.ChannelID,
		StartedAt: sess.StartedAt,
		EndedAt:   sess.EndedAt,
		Status:    sess.Status,
		Summary:   sess.Summary,
		KeyEvents: events,
		CreatedAt: sess.CreatedAt,
	}
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	guildID := r.URL.Query().Get("guild_id")
	if guildID == "" {
		guildID = s.guildID
	}

	sessions, err := s.store.ListSessions(r.Context(), guildID, limit, offset)
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
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
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

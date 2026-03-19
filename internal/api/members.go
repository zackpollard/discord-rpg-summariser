package api

import (
	"net/http"

	"discord-rpg-summariser/internal/bot"
)

// MemberProvider supplies Discord guild member data. Implemented by *bot.Bot.
type MemberProvider interface {
	GuildMembers() []bot.MemberInfo
	ResolveUsername(userID string) string
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	if s.memberP == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	members := s.memberP.GuildMembers()
	if members == nil {
		members = []bot.MemberInfo{}
	}
	writeJSON(w, http.StatusOK, members)
}

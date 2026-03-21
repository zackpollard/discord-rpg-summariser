package api

import (
	"net/http"

	"discord-rpg-summariser/internal/auth"
)

func (s *Server) setupRoutes() {
	// Auth endpoints (always public).
	s.mux.HandleFunc("GET /api/auth/login", s.handleAuthLogin)
	s.mux.HandleFunc("GET /api/auth/callback", s.handleAuthCallback)
	s.mux.HandleFunc("GET /api/auth/me", s.handleAuthMe)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleAuthLogout)

	// Protected API routes — wrapped with auth middleware when enabled.
	s.handle("GET /api/status", s.handleStatus)

	s.handle("GET /api/campaigns", s.handleListCampaigns)
	s.handle("POST /api/campaigns", s.handleCreateCampaign)
	s.handle("GET /api/campaigns/{id}", s.handleGetCampaign)
	s.handle("PUT /api/campaigns/{id}/active", s.handleSetActiveCampaign)
	s.handle("GET /api/campaigns/{id}/entities", s.handleListEntities)
	s.handle("GET /api/campaigns/{id}/quests", s.handleListQuests)
	s.handle("GET /api/campaigns/{id}/timeline", s.handleGetTimeline)
	s.handle("POST /api/campaigns/{id}/lore/ask", s.handleLoreAsk)
	s.handle("GET /api/campaigns/{id}/lore/search", s.handleLoreSearch)
	s.handle("GET /api/campaigns/{id}/recap", s.handleGetRecap)
	s.handle("POST /api/campaigns/{id}/recap", s.handleRegenerateRecap)
	s.handle("GET /api/campaigns/{id}/transcript-search", s.handleTranscriptSearch)
	s.handle("GET /api/campaigns/{id}/relationship-graph", s.handleRelationshipGraph)
	s.handle("GET /api/campaigns/{id}/location-hierarchy", s.handleLocationHierarchy)
	s.handle("GET /api/campaigns/{id}/entity-timeline", s.handleGetEntityTimeline)
	s.handle("GET /api/campaigns/{id}/stats", s.handleGetCampaignStats)
	s.handle("GET /api/campaigns/{id}/pdf", s.handleGetCampaignPDF)

	s.handle("GET /api/entities/{id}", s.handleGetEntity)
	s.handle("POST /api/entities/{id}/merge", s.handleMergeEntity)
	s.handle("GET /api/quests/{id}", s.handleGetQuest)

	s.handle("GET /api/sessions", s.handleListSessions)
	s.handle("GET /api/sessions/{id}", s.handleGetSession)
	s.handle("GET /api/sessions/{id}/transcript", s.handleGetTranscript)
	s.handle("POST /api/sessions/{id}/reprocess", s.handleReprocessSession)
	s.handle("GET /api/sessions/{id}/combat", s.handleGetSessionCombat)
	s.handle("GET /api/sessions/{id}/audio", s.handleGetSessionAudio)

	s.handle("GET /api/characters", s.handleListCharacters)
	s.handle("PUT /api/characters", s.handleUpsertCharacter)
	s.handle("DELETE /api/characters/{userId}", s.handleDeleteCharacter)

	s.handle("GET /api/members", s.handleListMembers)
	s.handle("GET /api/voice-activity", s.handleVoiceActivity)
	s.handle("GET /api/live-transcript", s.handleLiveTranscript)
}

// handle registers a route, wrapping the handler with auth middleware when
// authentication is enabled.
func (s *Server) handle(pattern string, handler http.HandlerFunc) {
	if s.authEnabled && s.sessions != nil {
		mw := auth.RequireAuth(s.sessions)
		s.mux.Handle(pattern, mw(handler))
	} else {
		s.mux.HandleFunc(pattern, handler)
	}
}

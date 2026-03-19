package api

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("GET /api/status", s.handleStatus)

	s.mux.HandleFunc("GET /api/campaigns", s.handleListCampaigns)
	s.mux.HandleFunc("POST /api/campaigns", s.handleCreateCampaign)
	s.mux.HandleFunc("GET /api/campaigns/{id}", s.handleGetCampaign)
	s.mux.HandleFunc("PUT /api/campaigns/{id}/active", s.handleSetActiveCampaign)
	s.mux.HandleFunc("GET /api/campaigns/{id}/entities", s.handleListEntities)
	s.mux.HandleFunc("GET /api/campaigns/{id}/quests", s.handleListQuests)
	s.mux.HandleFunc("GET /api/campaigns/{id}/timeline", s.handleGetTimeline)
	s.mux.HandleFunc("POST /api/campaigns/{id}/lore/ask", s.handleLoreAsk)
	s.mux.HandleFunc("GET /api/campaigns/{id}/lore/search", s.handleLoreSearch)
	s.mux.HandleFunc("GET /api/campaigns/{id}/recap", s.handleGetRecap)
	s.mux.HandleFunc("POST /api/campaigns/{id}/recap", s.handleRegenerateRecap)

	s.mux.HandleFunc("GET /api/entities/{id}", s.handleGetEntity)
	s.mux.HandleFunc("GET /api/quests/{id}", s.handleGetQuest)

	s.mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("GET /api/sessions/{id}/transcript", s.handleGetTranscript)
	s.mux.HandleFunc("POST /api/sessions/{id}/reprocess", s.handleReprocessSession)

	s.mux.HandleFunc("GET /api/characters", s.handleListCharacters)
	s.mux.HandleFunc("PUT /api/characters", s.handleUpsertCharacter)
	s.mux.HandleFunc("DELETE /api/characters/{userId}", s.handleDeleteCharacter)

	s.mux.HandleFunc("GET /api/members", s.handleListMembers)
	s.mux.HandleFunc("GET /api/voice-activity", s.handleVoiceActivity)
	s.mux.HandleFunc("GET /api/live-transcript", s.handleLiveTranscript)
}

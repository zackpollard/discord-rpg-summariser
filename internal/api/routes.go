package api

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("GET /api/status", s.handleStatus)

	s.mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("GET /api/sessions/{id}/transcript", s.handleGetTranscript)

	s.mux.HandleFunc("GET /api/characters", s.handleListCharacters)
	s.mux.HandleFunc("PUT /api/characters", s.handleUpsertCharacter)
	s.mux.HandleFunc("DELETE /api/characters/{userId}", s.handleDeleteCharacter)

	s.mux.HandleFunc("GET /api/voice-activity", s.handleVoiceActivity)
	s.mux.HandleFunc("GET /api/live-transcript", s.handleLiveTranscript)
}

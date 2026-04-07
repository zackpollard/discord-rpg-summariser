package api

import (
	"crypto/subtle"
	"log"
	"net/http"
	"time"

	"discord-rpg-summariser/internal/auth"
)

const (
	oauthStateCookie    = "oauth_state"
	oauthStateCookieAge = 10 * 60 // 10 minutes
)

// handleAuthLogin redirects to Discord's OAuth2 authorize page.
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.oauthCfg == nil {
		writeError(w, http.StatusServiceUnavailable, "OAuth not configured")
		return
	}

	state, err := auth.GenerateState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   oauthStateCookieAge,
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, s.oauthCfg.AuthorizeURL(state), http.StatusTemporaryRedirect)
}

// handleAuthCallback handles the OAuth2 callback from Discord.
func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.oauthCfg == nil {
		writeError(w, http.StatusServiceUnavailable, "OAuth not configured")
		return
	}

	// Verify state parameter for CSRF protection.
	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing state cookie")
		return
	}
	stateParam := r.URL.Query().Get("state")
	if subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(stateParam)) != 1 {
		writeError(w, http.StatusBadRequest, "invalid state parameter")
		return
	}

	// Clear the state cookie.
	http.SetCookie(w, &http.Cookie{
		Name:   oauthStateCookie,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Check for error from Discord.
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		log.Printf("OAuth error from Discord: %s - %s", errParam, r.URL.Query().Get("error_description"))
		http.Redirect(w, r, "/login?error=access_denied", http.StatusTemporaryRedirect)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing authorization code")
		return
	}

	// Exchange code for access token.
	accessToken, err := s.oauthCfg.ExchangeCode(code)
	if err != nil {
		log.Printf("OAuth token exchange failed: %v", err)
		writeError(w, http.StatusBadGateway, "token exchange failed")
		return
	}

	// Fetch user info.
	user, err := auth.FetchUser(accessToken)
	if err != nil {
		log.Printf("Failed to fetch Discord user: %v", err)
		writeError(w, http.StatusBadGateway, "failed to fetch user info")
		return
	}

	// Verify guild membership.
	isMember, err := auth.IsGuildMember(accessToken, s.guildID)
	if err != nil {
		log.Printf("Failed to check guild membership for user %s: %v", user.ID, err)
		writeError(w, http.StatusBadGateway, "failed to verify guild membership")
		return
	}
	if !isMember {
		log.Printf("User %s (%s) is not a member of guild %s", user.ID, user.Username, s.guildID)
		http.Redirect(w, r, "/login?error=not_member", http.StatusTemporaryRedirect)
		return
	}

	// Create session.
	sessionData := &auth.SessionData{
		UserID:    user.ID,
		Username:  user.Username,
		Avatar:    user.Avatar,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour).Unix(),
	}

	if err := s.sessions.Encode(w, sessionData); err != nil {
		log.Printf("Failed to create session for user %s: %v", user.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	log.Printf("User %s (%s) logged in via Discord OAuth", user.ID, user.Username)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// handleAuthMe returns the current authenticated user.
func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	// When auth is disabled, return a dummy user so the frontend works.
	if !s.authEnabled || s.sessions == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"id":       "0",
			"username": "local",
			"avatar":   "",
		})
		return
	}

	session, err := s.sessions.Decode(r)
	if err != nil || session == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":       session.UserID,
		"username": session.Username,
		"avatar":   session.Avatar,
	})
}

// handleAuthLogout clears the session cookie.
func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	s.sessions.Clear(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

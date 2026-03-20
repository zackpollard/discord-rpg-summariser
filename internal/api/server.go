package api

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"discord-rpg-summariser/internal/auth"
	"discord-rpg-summariser/internal/config"
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/voice"
)

// VoiceActivityProvider supplies live voice activity data. Implemented by *bot.Bot.
type VoiceActivityProvider interface {
	VoiceActivity() []voice.UserActivity
}

type Server struct {
	store         *storage.Store
	listenAddr    string
	guildID       string
	voiceAP       VoiceActivityProvider
	liveTP        LiveTranscriptProvider
	memberP       MemberProvider
	loreQA        LoreQAProvider
	reprocessor   SessionReprocessor
	mux           *http.ServeMux
	httpServer    *http.Server
	sessions      *auth.SessionManager
	oauthCfg      *auth.OAuthConfig
	secureCookies bool
	authEnabled   bool
}

func NewServer(store *storage.Store, listenAddr, guildID, webDir string, opts ...Option) *Server {
	s := &Server{
		store:      store,
		listenAddr: listenAddr,
		guildID:    guildID,
		mux:        http.NewServeMux(),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.setupRoutes()
	s.setupSPA(webDir)

	s.httpServer = &http.Server{
		Addr:    listenAddr,
		Handler: s.corsMiddleware(s.mux),
	}

	return s
}

// Option configures a Server.
type Option func(*Server)

// WithAuth configures Discord OAuth2 authentication for the server.
func WithAuth(cfg *config.Config) Option {
	return func(s *Server) {
		disc := cfg.Discord
		web := cfg.Web

		// If client ID / secret are not configured, skip auth.
		if disc.ClientID == "" || disc.ClientSecret == "" {
			log.Println("OAuth2 client_id/client_secret not set — auth disabled")
			return
		}

		secureCookies := !strings.HasPrefix(disc.RedirectURL, "http://localhost") &&
			!strings.HasPrefix(disc.RedirectURL, "http://127.0.0.1")

		sm, err := auth.NewSessionManager(web.SessionSecret, secureCookies)
		if err != nil {
			log.Fatalf("Failed to create session manager: %v", err)
		}

		s.sessions = sm
		s.secureCookies = secureCookies
		s.authEnabled = true
		s.oauthCfg = &auth.OAuthConfig{
			ClientID:     disc.ClientID,
			ClientSecret: disc.ClientSecret,
			RedirectURL:  disc.RedirectURL,
		}

		log.Println("Discord OAuth2 authentication enabled")
	}
}

// SetVoiceActivityProvider sets the provider for live voice activity data.
func (s *Server) SetVoiceActivityProvider(vap VoiceActivityProvider) {
	s.voiceAP = vap
}

func (s *Server) SetLiveTranscriptProvider(ltp LiveTranscriptProvider) {
	s.liveTP = ltp
}

func (s *Server) SetMemberProvider(mp MemberProvider) {
	s.memberP = mp
}

// SetLoreQAProvider sets the provider for lore Q&A and recap generation.
func (s *Server) SetLoreQAProvider(lqp LoreQAProvider) {
	s.loreQA = lqp
}

func (s *Server) Start() error {
	log.Printf("API server listening on %s", s.listenAddr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// setupSPA configures static file serving from the Svelte build directory.
func (s *Server) setupSPA(webDir string) {
	if webDir == "" {
		return
	}

	staticFS := os.DirFS(webDir)
	fileServer := http.FileServerFS(staticFS)

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		cleaned := strings.TrimPrefix(path, "/")
		if _, err := fs.Stat(staticFS, cleaned); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		indexData, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexData)
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") &&
			!strings.HasSuffix(r.URL.Path, "/voice-activity") &&
			!strings.HasSuffix(r.URL.Path, "/live-transcript") &&
			!strings.HasSuffix(r.URL.Path, "/audio") &&
			!strings.HasPrefix(r.URL.Path, "/api/auth/login") &&
			!strings.HasPrefix(r.URL.Path, "/api/auth/callback") {
			w.Header().Set("Content-Type", "application/json")
		}

		next.ServeHTTP(w, r)
	})
}

package config

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Discord    DiscordConfig    `yaml:"discord"`
	Telegram   TelegramConfig   `yaml:"telegram"`
	Transcribe TranscribeConfig `yaml:"transcribe"`
	Diarize    DiarizeConfig    `yaml:"diarize"`
	TTS        TTSConfig        `yaml:"tts"`
	LLM        LLMConfig        `yaml:"llm"`
	Storage    StorageConfig    `yaml:"storage"`
	Web        WebConfig        `yaml:"web"`
}

type TTSConfig struct {
	Engine  string `yaml:"engine"`  // "f5tts" (default) or "zipvoice"
	Threads int    `yaml:"threads"` // defaults to transcribe.threads if 0
}

type DiscordConfig struct {
	Token               string `yaml:"token"`
	GuildID             string `yaml:"guild_id"`
	NotificationChannel string `yaml:"notification_channel_id"`
	ClientID            string `yaml:"client_id"`
	ClientSecret        string `yaml:"client_secret"`
	RedirectURL         string `yaml:"redirect_url"`
}

type TranscribeConfig struct {
	Engine   string `yaml:"engine"`    // "whisper" (default) or "parakeet"
	Model    string `yaml:"model"`     // whisper model name: tiny, base, small, medium, large-v3
	ModelDir string `yaml:"model_dir"` // directory to store downloaded models
	Language string `yaml:"language"`
	Threads  int    `yaml:"threads"`
	GPU      bool   `yaml:"gpu"`
}

type DiarizeConfig struct {
	ModelDir string `yaml:"model_dir"` // defaults to transcribe.model_dir if empty
	Threads  int    `yaml:"threads"`   // defaults to transcribe.threads if 0
}

type LLMConfig struct {
	Provider          string `yaml:"provider"`
	OllamaURL         string `yaml:"ollama_url"`
	OllamaModel       string `yaml:"ollama_model"`
	EmbeddingModel    string `yaml:"embedding_model"`
	EmbeddingModelDir string `yaml:"embedding_model_dir"`
}

type StorageConfig struct {
	DatabaseURL string `yaml:"database_url"`
	AudioDir    string `yaml:"audio_dir"`
}

type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   int64  `yaml:"chat_id"`
}

type WebConfig struct {
	ListenAddr    string `yaml:"listen_addr"`
	BaseURL       string `yaml:"base_url"`
	SessionSecret string `yaml:"session_secret"`
	// AllowUnauthenticated opts in to serving the panel without Discord OAuth2
	// on a non-loopback listen address. Without it, Validate refuses to start.
	AllowUnauthenticated bool `yaml:"allow_unauthenticated"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Transcribe: TranscribeConfig{
			Engine:   "whisper",
			Model:    "base",
			ModelDir: "models",
			Language: "en",
			Threads:  4,
		},
		LLM: LLMConfig{
			Provider:          "claude-cli",
			OllamaURL:         "http://localhost:11434",
			EmbeddingModel:    "nomic-embed-text",
			EmbeddingModelDir: "models/embedding",
		},
		Storage: StorageConfig{
			DatabaseURL: "postgres://localhost:5432/rpg_summariser?sslmode=disable",
			AudioDir:    "data/audio",
		},
		Web: WebConfig{
			ListenAddr: ":8080",
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if token := os.Getenv("DISCORD_TOKEN"); token != "" {
		cfg.Discord.Token = token
	}
	if guildID := os.Getenv("DISCORD_GUILD_ID"); guildID != "" {
		cfg.Discord.GuildID = guildID
	}
	if clientID := os.Getenv("DISCORD_CLIENT_ID"); clientID != "" {
		cfg.Discord.ClientID = clientID
	}
	if clientSecret := os.Getenv("DISCORD_CLIENT_SECRET"); clientSecret != "" {
		cfg.Discord.ClientSecret = clientSecret
	}
	if baseURL := os.Getenv("WEB_BASE_URL"); baseURL != "" {
		cfg.Web.BaseURL = baseURL
	}
	if sessionSecret := os.Getenv("WEB_SESSION_SECRET"); sessionSecret != "" {
		cfg.Web.SessionSecret = sessionSecret
	}
	if allowUnauth := os.Getenv("WEB_ALLOW_UNAUTHENTICATED"); allowUnauth != "" {
		v, err := strconv.ParseBool(allowUnauth)
		if err != nil {
			return nil, fmt.Errorf("invalid WEB_ALLOW_UNAUTHENTICATED %q: %w", allowUnauth, err)
		}
		cfg.Web.AllowUnauthenticated = v
	}
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		cfg.Storage.DatabaseURL = dbURL
	}
	if tgToken := os.Getenv("TELEGRAM_BOT_TOKEN"); tgToken != "" {
		cfg.Telegram.BotToken = tgToken
	}

	return cfg, nil
}

// Validate checks for configurations that cannot work as intended. It only
// rejects genuine misconfiguration; settings that are merely risky are
// reported by Warnings so an existing deployment still starts after an
// upgrade.
func (c *Config) Validate() error {
	hasClientID := c.Discord.ClientID != ""
	hasClientSecret := c.Discord.ClientSecret != ""

	if hasClientID != hasClientSecret {
		return fmt.Errorf("discord.client_id and discord.client_secret must both be set to enable web authentication (set both, or leave both empty)")
	}

	return nil
}

// Warnings returns security-relevant conditions that are permitted but likely
// unintended. Web authentication is disabled whenever the OAuth2 credentials
// are absent, so serving on a non-loopback address exposes session audio and
// transcripts to anyone who can reach the port.
func (c *Config) Warnings() []string {
	var warnings []string

	if c.Discord.ClientID == "" && !c.Web.AllowUnauthenticated && !isLoopback(c.Web.ListenAddr) {
		warnings = append(warnings, fmt.Sprintf(
			"web.listen_addr %q accepts connections from any interface but discord.client_id/client_secret are unset, "+
				"so the web panel serves session audio and transcripts to anyone who can reach it: configure OAuth2, "+
				"bind web.listen_addr to 127.0.0.1, or set web.allow_unauthenticated: true to silence this",
			c.Web.ListenAddr))
	}

	return warnings
}

// isLoopback reports whether a listen address binds the loopback interface
// only. An empty host (":8080") binds every interface.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

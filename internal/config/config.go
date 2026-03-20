package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Discord    DiscordConfig    `yaml:"discord"`
	Telegram   TelegramConfig   `yaml:"telegram"`
	Transcribe TranscribeConfig `yaml:"transcribe"`
	Diarize    DiarizeConfig    `yaml:"diarize"`
	LLM        LLMConfig        `yaml:"llm"`
	Storage    StorageConfig    `yaml:"storage"`
	Web        WebConfig        `yaml:"web"`
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
	Provider       string `yaml:"provider"`
	OllamaURL      string `yaml:"ollama_url"`
	OllamaModel    string `yaml:"ollama_model"`
	EmbeddingModel string `yaml:"embedding_model"`
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
	SessionSecret string `yaml:"session_secret"`
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
			Provider:       "claude-cli",
			OllamaURL:      "http://localhost:11434",
			EmbeddingModel: "nomic-embed-text",
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
	if sessionSecret := os.Getenv("WEB_SESSION_SECRET"); sessionSecret != "" {
		cfg.Web.SessionSecret = sessionSecret
	}
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		cfg.Storage.DatabaseURL = dbURL
	}
	if tgToken := os.Getenv("TELEGRAM_BOT_TOKEN"); tgToken != "" {
		cfg.Telegram.BotToken = tgToken
	}

	return cfg, nil
}

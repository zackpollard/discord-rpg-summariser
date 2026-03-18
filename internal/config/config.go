package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Discord DiscordConfig `yaml:"discord"`
	Whisper WhisperConfig `yaml:"whisper"`
	LLM     LLMConfig     `yaml:"llm"`
	Storage StorageConfig `yaml:"storage"`
	Web     WebConfig     `yaml:"web"`
}

type DiscordConfig struct {
	Token               string `yaml:"token"`
	GuildID             string `yaml:"guild_id"`
	NotificationChannel string `yaml:"notification_channel_id"`
}

type WhisperConfig struct {
	BinaryPath string `yaml:"binary_path"`
	ModelPath  string `yaml:"model_path"`
	Threads    int    `yaml:"threads"`
	Language   string `yaml:"language"`
	GPU        bool   `yaml:"gpu"`
}

type LLMConfig struct {
	Provider    string `yaml:"provider"`
	OllamaURL   string `yaml:"ollama_url"`
	OllamaModel string `yaml:"ollama_model"`
}

type StorageConfig struct {
	DatabaseURL string `yaml:"database_url"`
	AudioDir    string `yaml:"audio_dir"`
}

type WebConfig struct {
	ListenAddr string `yaml:"listen_addr"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Whisper: WhisperConfig{
			BinaryPath: "whisper-cli",
			Threads:    4,
			Language:   "en",
		},
		LLM: LLMConfig{
			Provider:  "claude-cli",
			OllamaURL: "http://localhost:11434",
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
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		cfg.Storage.DatabaseURL = dbURL
	}

	return cfg, nil
}

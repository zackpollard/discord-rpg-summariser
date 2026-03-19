package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	yaml := `
discord:
  token: "test-token"
  guild_id: "123456"
  notification_channel_id: "789"

transcribe:
  model: "large-v3"
  model_dir: "/models"
  language: "en"
  threads: 8
  gpu: true

llm:
  provider: "ollama"
  ollama_url: "http://localhost:11434"
  ollama_model: "llama3"

storage:
  database_url: "postgres://localhost:5432/test?sslmode=disable"
  audio_dir: "/tmp/audio"

web:
  listen_addr: ":9090"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Discord.Token != "test-token" {
		t.Errorf("Discord.Token = %q, want %q", cfg.Discord.Token, "test-token")
	}
	if cfg.Discord.GuildID != "123456" {
		t.Errorf("Discord.GuildID = %q, want %q", cfg.Discord.GuildID, "123456")
	}
	if cfg.Transcribe.Threads != 8 {
		t.Errorf("Transcribe.Threads = %d, want 8", cfg.Transcribe.Threads)
	}
	if !cfg.Transcribe.GPU {
		t.Error("Transcribe.GPU = false, want true")
	}
	if cfg.Transcribe.Model != "large-v3" {
		t.Errorf("Transcribe.Model = %q, want %q", cfg.Transcribe.Model, "large-v3")
	}
	if cfg.LLM.Provider != "ollama" {
		t.Errorf("LLM.Provider = %q, want %q", cfg.LLM.Provider, "ollama")
	}
	if cfg.Storage.DatabaseURL != "postgres://localhost:5432/test?sslmode=disable" {
		t.Errorf("Storage.DatabaseURL = %q, want postgres URL", cfg.Storage.DatabaseURL)
	}
	if cfg.Web.ListenAddr != ":9090" {
		t.Errorf("Web.ListenAddr = %q, want %q", cfg.Web.ListenAddr, ":9090")
	}
}

func TestLoadDefaults(t *testing.T) {
	yaml := `
discord:
  token: "tok"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Transcribe.Threads != 4 {
		t.Errorf("default Transcribe.Threads = %d, want 4", cfg.Transcribe.Threads)
	}
	if cfg.Transcribe.Model != "base" {
		t.Errorf("default Transcribe.Model = %q, want %q", cfg.Transcribe.Model, "base")
	}
	if cfg.LLM.Provider != "claude-cli" {
		t.Errorf("default LLM.Provider = %q, want %q", cfg.LLM.Provider, "claude-cli")
	}
	if cfg.Web.ListenAddr != ":8080" {
		t.Errorf("default Web.ListenAddr = %q, want %q", cfg.Web.ListenAddr, ":8080")
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	yaml := `
discord:
  token: "yaml-token"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DISCORD_TOKEN", "env-token")
	t.Setenv("DATABASE_URL", "postgres://env:5432/db")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Discord.Token != "env-token" {
		t.Errorf("Discord.Token = %q, want %q (env override)", cfg.Discord.Token, "env-token")
	}
	if cfg.Storage.DatabaseURL != "postgres://env:5432/db" {
		t.Errorf("Storage.DatabaseURL = %q, want env override", cfg.Storage.DatabaseURL)
	}
}

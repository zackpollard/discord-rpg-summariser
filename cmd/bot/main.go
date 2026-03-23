package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"discord-rpg-summariser/internal/api"
	"discord-rpg-summariser/internal/bot"
	"discord-rpg-summariser/internal/config"
	"discord-rpg-summariser/internal/embed"
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/telegram"
	"discord-rpg-summariser/internal/transcribe"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	log.Printf("Discord RPG Summariser %s", version)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	migrationsDir := os.DirFS("migrations")
	store, err := storage.New(ctx, cfg.Storage.DatabaseURL, migrationsDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()
	log.Println("Database connected and migrated")

	var transcriber transcribe.Transcriber
	switch cfg.Transcribe.Engine {
	case "parakeet":
		transcriber, err = transcribe.NewParakeetTranscriber(
			cfg.Transcribe.ModelDir,
			cfg.Transcribe.Threads,
		)
	default:
		transcriber, err = transcribe.NewWhisperTranscriber(
			cfg.Transcribe.Model,
			cfg.Transcribe.ModelDir,
			cfg.Transcribe.Language,
			cfg.Transcribe.Threads,
		)
	}
	if err != nil {
		log.Fatalf("Failed to initialize transcriber: %v", err)
	}
	defer transcriber.Close()
	log.Printf("Transcription engine loaded: %s", cfg.Transcribe.Engine)

	var sum summarise.Summariser
	switch cfg.LLM.Provider {
	case "ollama":
		sum = summarise.NewOllama(cfg.LLM.OllamaURL, cfg.LLM.OllamaModel)
	default:
		sum = summarise.NewClaudeCLI()
	}

	webDir := "web/build"
	if env := os.Getenv("WEB_DIR"); env != "" {
		webDir = env
	}

	if cfg.Telegram.BotToken != "" {
		log.Println("Telegram integration enabled")
	}

	srv := api.NewServer(store, cfg.Web.ListenAddr, cfg.Discord.GuildID, webDir, api.WithAuth(cfg))

	discordBot, err := bot.NewBot(cfg, store, transcriber, sum)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	if cfg.Telegram.BotToken != "" {
		discordBot.SetTelegramClient(telegram.NewClient(cfg.Telegram.BotToken))
	}

	// Set up embedder for RAG if Ollama URL and embedding model are configured.
	var embedder embed.Embedder
	if cfg.LLM.OllamaURL != "" && cfg.LLM.EmbeddingModel != "" {
		embedder = embed.NewOllamaEmbedder(cfg.LLM.OllamaURL, cfg.LLM.EmbeddingModel)
		discordBot.SetEmbedder(embedder)
		log.Printf("Embedding model enabled: %s via %s", cfg.LLM.EmbeddingModel, cfg.LLM.OllamaURL)
	}

	srv.SetVoiceActivityProvider(discordBot)
	srv.SetLiveTranscriptProvider(discordBot)
	srv.SetMemberProvider(discordBot)
	srv.SetLoreQAProvider(discordBot)
	srv.SetSessionReprocessor(discordBot)
	if embedder != nil {
		srv.SetEmbedder(embedder)
	}

	go func() {
		log.Printf("API server listening on %s", cfg.Web.ListenAddr)
		if err := srv.Start(); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	if err := discordBot.Start(); err != nil {
		log.Fatalf("Failed to start bot: %v", err)
	}
	log.Println("Bot is running. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	cancel()

	if err := discordBot.Stop(); err != nil {
		log.Printf("Error stopping bot: %v", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error shutting down API server: %v", err)
	}

	log.Println("Shutdown complete")
}

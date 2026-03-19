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
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/transcribe"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

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

	transcriber, err := transcribe.NewTranscriber(
		cfg.Transcribe.Model,
		cfg.Transcribe.ModelDir,
		cfg.Transcribe.Language,
		cfg.Transcribe.Threads,
	)
	if err != nil {
		log.Fatalf("Failed to initialize transcriber: %v", err)
	}
	defer transcriber.Close()
	log.Println("Whisper model loaded")

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

	srv := api.NewServer(store, cfg.Web.ListenAddr, cfg.Discord.GuildID, webDir)

	discordBot, err := bot.NewBot(cfg, store, transcriber, sum)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	srv.SetVoiceActivityProvider(discordBot)

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

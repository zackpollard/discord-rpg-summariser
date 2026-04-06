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
	"discord-rpg-summariser/internal/tts"
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

	// Create a factory that loads the transcription model on demand.
	// This avoids keeping ~22GB of ONNX Runtime memory allocated while idle.
	transcriberFactory := func() (transcribe.Transcriber, error) {
		switch cfg.Transcribe.Engine {
		case "parakeet":
			return transcribe.NewParakeetTranscriber(
				cfg.Transcribe.ModelDir,
				cfg.Transcribe.Threads,
			)
		default:
			return transcribe.NewWhisperTranscriber(
				cfg.Transcribe.Model,
				cfg.Transcribe.ModelDir,
				cfg.Transcribe.Language,
				cfg.Transcribe.Threads,
			)
		}
	}
	log.Printf("Transcription engine configured: %s (lazy-loaded)", cfg.Transcribe.Engine)

	var sum summarise.Summariser
	var claudeCLI *summarise.ClaudeCLI
	switch cfg.LLM.Provider {
	case "ollama":
		sum = summarise.NewOllama(cfg.LLM.OllamaURL, cfg.LLM.OllamaModel)
	default:
		cli := summarise.NewClaudeCLI()
		cli.OnLog = func(ctx context.Context, entry summarise.LLMLogEntry) {
			sessionID := summarise.SessionIDFromContext(ctx)
			var sid *int64
			if sessionID != 0 {
				sid = &sessionID
			}
			var errStr *string
			if entry.Error != "" {
				errStr = &entry.Error
			}
			if _, err := store.InsertLLMLog(ctx, storage.LLMLog{
				SessionID:  sid,
				Operation:  entry.Operation,
				Prompt:     entry.Prompt,
				Response:   entry.Response,
				Error:      errStr,
				DurationMS: entry.DurationMS,
			}); err != nil {
				log.Printf("Failed to save LLM log: %v", err)
			}
		}
		claudeCLI = cli
		sum = cli
	}

	webDir := "web/build"
	if env := os.Getenv("WEB_DIR"); env != "" {
		webDir = env
	}

	if cfg.Telegram.BotToken != "" {
		log.Println("Telegram integration enabled")
	}

	srv := api.NewServer(store, cfg.Web.ListenAddr, cfg.Discord.GuildID, webDir, api.WithAuth(cfg))

	discordBot, err := bot.NewBot(cfg, store, transcriberFactory, sum)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	if cfg.Telegram.BotToken != "" {
		discordBot.SetTelegramClient(telegram.NewClient(cfg.Telegram.BotToken))
	}

	// Set up in-process ONNX embedding model.
	var embedder embed.Embedder
	onnxEmb, err := embed.NewOnnxEmbedder(cfg.LLM.EmbeddingModelDir, cfg.Transcribe.Threads)
	if err != nil {
		log.Printf("Warning: embedding model unavailable: %v", err)
	} else {
		embedder = onnxEmb
		discordBot.SetEmbedder(embedder)
		defer onnxEmb.Close()
		log.Println("Embedding model enabled: in-process ONNX (nomic-embed-text-v1.5)")
	}

	// Set up TTS service (Python subprocess).
	ttsThreads := cfg.TTS.Threads
	if ttsThreads == 0 {
		ttsThreads = cfg.Transcribe.Threads
	}
	projectDir, _ := os.Getwd()
	ttsSynth, err := tts.NewSynthesizer(projectDir, ttsThreads, cfg.TTS.Engine)
	if err != nil {
		log.Printf("Warning: TTS unavailable: %v", err)
	} else {
		srv.SetTTSService(api.NewTTSService(ttsSynth, store))
		discordBot.SetTTSSynthesizer(ttsSynth)
		defer ttsSynth.Close()
		log.Println("TTS enabled: ZipVoice (voice-cloned recap)")
	}

	if claudeCLI != nil {
		srv.SetSummariser(claudeCLI)
	}
	srv.SetSoundboardPlayer(discordBot)
	srv.SetVoiceActivityProvider(discordBot)
	srv.SetLiveTranscriptProvider(discordBot)
	srv.SetPipelineProgressProvider(discordBot)
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

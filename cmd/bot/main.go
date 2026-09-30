package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"video_download_bot/internal/adapter/instagram"
	"video_download_bot/internal/adapter/postgres"
	"video_download_bot/internal/config"
	"video_download_bot/internal/delivery/telegram"
	"video_download_bot/internal/usecase"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}
	if err := postgres.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}
	api.Debug = cfg.Debug
	log.Printf("authorized as @%s", api.Self.UserName)

	members := usecase.NewMembershipService(postgres.NewChatRepository(pool))
	media := usecase.NewMediaService(
		instagram.NewReelDownloader(cfg.DownloadTimeout),
		instagram.NewPostFetcher(cfg.PostFetchTimeout),
		postgres.NewDownloadRepository(pool),
	)

	telegram.NewHandler(api, media, members, cfg.MaxConcurrent).Run(ctx)
	log.Println("stopped")
}

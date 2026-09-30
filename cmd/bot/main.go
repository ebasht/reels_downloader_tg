package main

import (
	"context"
	"log"
	"net"
	"net/url"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"video_download_bot/internal/adapter/instagram"
	"video_download_bot/internal/adapter/listing"
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
	warnInsecureDatabase(cfg.DatabaseURL)

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

	api, err := telegram.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}
	api.Debug = cfg.Debug
	log.Printf("authorized as @%s", api.Self.UserName)

	members := usecase.NewMembershipService(postgres.NewChatRepository(pool))
	media := usecase.NewMediaService(
		instagram.NewReelDownloader(cfg.DownloadTimeout, cfg.MaxVideoDuration),
		instagram.NewPostFetcher(cfg.PostFetchTimeout),
		listing.NewFetcher(cfg.PostFetchTimeout),
		postgres.NewDownloadRepository(pool),
	)

	telegram.NewHandler(api, media, members, telegram.Limits{
		MaxConcurrent: cfg.MaxConcurrent,
		MaxQueue:      cfg.MaxQueue,
		PerUser:       usecase.NewRateLimiter(cfg.UserRateLimit, cfg.RateWindow),
		PerChat:       usecase.NewRateLimiter(cfg.ChatRateLimit, cfg.RateWindow),
	}).Run(ctx)
	log.Println("stopped")
}

func warnInsecureDatabase(databaseURL string) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return
	}
	host := u.Hostname()
	if host == "localhost" || host == "postgres" {
		return
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return
	}
	switch u.Query().Get("sslmode") {
	case "require", "verify-ca", "verify-full":
		return
	}
	log.Printf("WARNING: database %s is remote but TLS is not required; set sslmode=verify-full", host)
}

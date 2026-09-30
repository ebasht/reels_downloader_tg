package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	BotToken         string
	DatabaseURL      string
	MaxConcurrent    int
	DownloadTimeout  time.Duration
	PostFetchTimeout time.Duration
	Debug            bool
}

func Load() (Config, error) {
	cfg := Config{
		BotToken:         os.Getenv("BOT"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		MaxConcurrent:    3,
		DownloadTimeout:  3 * time.Minute,
		PostFetchTimeout: 30 * time.Second,
		Debug:            os.Getenv("BOT_DEBUG") == "true",
	}
	if cfg.BotToken == "" {
		return cfg, errors.New("BOT (telegram bot token) is required")
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL is required")
	}

	if v := os.Getenv("MAX_CONCURRENT_DOWNLOADS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return cfg, fmt.Errorf("invalid MAX_CONCURRENT_DOWNLOADS %q", v)
		}
		cfg.MaxConcurrent = n
	}
	if v := os.Getenv("DOWNLOAD_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return cfg, fmt.Errorf("invalid DOWNLOAD_TIMEOUT %q", v)
		}
		cfg.DownloadTimeout = d
	}
	return cfg, nil
}

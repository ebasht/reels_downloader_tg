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
	MaxQueue         int
	UserRateLimit    int
	ChatRateLimit    int
	RateWindow       time.Duration
	MaxVideoDuration time.Duration
	DownloadTimeout  time.Duration
	PostFetchTimeout time.Duration
	Debug            bool
}

func Load() (Config, error) {
	cfg := Config{
		BotToken:         os.Getenv("BOT"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		MaxConcurrent:    3,
		MaxQueue:         30,
		UserRateLimit:    5,
		ChatRateLimit:    20,
		RateWindow:       time.Minute,
		MaxVideoDuration: 10 * time.Minute,
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

	ints := []struct {
		name string
		dst  *int
	}{
		{"MAX_CONCURRENT_DOWNLOADS", &cfg.MaxConcurrent},
		{"MAX_QUEUE", &cfg.MaxQueue},
		{"USER_RATE_LIMIT", &cfg.UserRateLimit},
		{"CHAT_RATE_LIMIT", &cfg.ChatRateLimit},
	}
	for _, v := range ints {
		if err := parsePositiveInt(v.name, v.dst); err != nil {
			return cfg, err
		}
	}

	durations := []struct {
		name string
		dst  *time.Duration
	}{
		{"RATE_WINDOW", &cfg.RateWindow},
		{"MAX_VIDEO_DURATION", &cfg.MaxVideoDuration},
		{"DOWNLOAD_TIMEOUT", &cfg.DownloadTimeout},
	}
	for _, v := range durations {
		if err := parsePositiveDuration(v.name, v.dst); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func parsePositiveInt(name string, dst *int) error {
	v := os.Getenv(name)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fmt.Errorf("invalid %s %q", name, v)
	}
	*dst = n
	return nil
}

func parsePositiveDuration(name string, dst *time.Duration) error {
	v := os.Getenv(name)
	if v == "" {
		return nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fmt.Errorf("invalid %s %q", name, v)
	}
	*dst = d
	return nil
}

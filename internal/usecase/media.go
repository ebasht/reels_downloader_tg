package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"video_download_bot/internal/domain"
)

type ReelDownloader interface {
	DownloadReel(ctx context.Context, url string) (*domain.Video, error)
}

type PostFetcher interface {
	// FetchPost returns domain.ErrPostIsVideo when the post is a video.
	FetchPost(ctx context.Context, url string) (*domain.Post, error)
}

type DownloadRepository interface {
	// SaveDownload stores the link and increments the chat's counter for its type.
	SaveDownload(ctx context.Context, d domain.Download) error
}

type MediaService struct {
	reels     ReelDownloader
	posts     PostFetcher
	downloads DownloadRepository
}

func NewMediaService(reels ReelDownloader, posts PostFetcher, downloads DownloadRepository) *MediaService {
	return &MediaService{reels: reels, posts: posts, downloads: downloads}
}

// RecordDelivered saves media that was successfully sent to a chat.
func (s *MediaService) RecordDelivered(ctx context.Context, chatID int64, user domain.User, link domain.InstagramLink, media domain.Media) error {
	d := domain.Download{
		ChatID:    chatID,
		User:      user,
		URL:       link.URL,
		Shortcode: link.Shortcode,
		Type:      media.Type(),
		At:        time.Now(),
	}
	if err := s.downloads.SaveDownload(ctx, d); err != nil {
		return fmt.Errorf("save download %s in chat %d: %w", link.URL, chatID, err)
	}
	return nil
}

// Fetch downloads the media behind link. The caller must call Media.Release.
func (s *MediaService) Fetch(ctx context.Context, link domain.InstagramLink) (domain.Media, error) {
	switch link.Kind {
	case domain.LinkReel:
		return s.fetchVideo(ctx, link.URL)
	case domain.LinkPost:
		post, err := s.posts.FetchPost(ctx, link.URL)
		if errors.Is(err, domain.ErrPostIsVideo) {
			return s.fetchVideo(ctx, link.URL)
		}
		if err != nil {
			return domain.Media{}, fmt.Errorf("fetch post %s: %w", link.URL, err)
		}
		return domain.Media{Post: post}, nil
	default:
		return domain.Media{}, fmt.Errorf("unsupported link kind %d", link.Kind)
	}
}

func (s *MediaService) fetchVideo(ctx context.Context, url string) (domain.Media, error) {
	video, err := s.reels.DownloadReel(ctx, url)
	if err != nil {
		return domain.Media{}, fmt.Errorf("download reel %s: %w", url, err)
	}
	return domain.Media{Video: video}, nil
}

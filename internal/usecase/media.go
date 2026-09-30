package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
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

type ListingFetcher interface {
	FetchListing(ctx context.Context, link domain.Link) (*domain.Listing, error)
}

type DownloadRepository interface {
	// SaveDownload stores the link and increments the chat's counter for its type.
	SaveDownload(ctx context.Context, d domain.Download) error
	// FindCached returns media previously sent to Telegram for the shortcode.
	FindCached(ctx context.Context, shortcode string) (domain.Media, bool, error)
	SaveCached(ctx context.Context, shortcode string, m domain.Media) error
	DeleteCached(ctx context.Context, shortcode string) error
}

type MediaService struct {
	reels     ReelDownloader
	posts     PostFetcher
	listings  ListingFetcher
	downloads DownloadRepository
}

func NewMediaService(reels ReelDownloader, posts PostFetcher, listings ListingFetcher, downloads DownloadRepository) *MediaService {
	return &MediaService{reels: reels, posts: posts, listings: listings, downloads: downloads}
}

// Fetch returns the media behind link, preferring files already sent to
// Telegram. The caller must call Media.Release.
func (s *MediaService) Fetch(ctx context.Context, link domain.Link) (domain.Media, error) {
	// Listings change (price, text), so they are always fetched fresh.
	if link.Kind == domain.LinkListing {
		return s.fetchListing(ctx, link)
	}

	cached, ok, err := s.downloads.FindCached(ctx, link.ID)
	if err != nil {
		log.Printf("media cache lookup: %v", err)
	}
	if ok {
		return cached, nil
	}
	return s.FetchFresh(ctx, link)
}

// FetchFresh downloads the media from Instagram, bypassing and invalidating
// the cache. Used when a cached Telegram file ID is no longer accepted.
func (s *MediaService) FetchFresh(ctx context.Context, link domain.Link) (domain.Media, error) {
	if err := s.downloads.DeleteCached(ctx, link.ID); err != nil {
		log.Printf("media cache invalidate: %v", err)
	}

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
	case domain.LinkListing:
		return s.fetchListing(ctx, link)
	default:
		return domain.Media{}, fmt.Errorf("unsupported link kind %d", link.Kind)
	}
}

func (s *MediaService) fetchListing(ctx context.Context, link domain.Link) (domain.Media, error) {
	l, err := s.listings.FetchListing(ctx, link)
	if err != nil {
		return domain.Media{}, fmt.Errorf("fetch listing %s: %w", link.URL, err)
	}
	return domain.Media{Post: &domain.Post{Images: l.Images, Caption: l.Caption()}}, nil
}

func (s *MediaService) fetchVideo(ctx context.Context, url string) (domain.Media, error) {
	video, err := s.reels.DownloadReel(ctx, url)
	if err != nil {
		return domain.Media{}, fmt.Errorf("download reel %s: %w", url, err)
	}
	return domain.Media{Video: video}, nil
}

// RecordDelivered saves media that was successfully sent to a chat. media must
// carry the Telegram file ID returned by the send.
func (s *MediaService) RecordDelivered(ctx context.Context, chatID int64, user domain.User, link domain.Link, media domain.Media) error {
	d := domain.Download{
		ChatID:    chatID,
		User:      user,
		URL:       link.URL,
		Shortcode: link.ID,
		Type:      media.Type(),
		At:        time.Now(),
	}
	if link.Kind == domain.LinkListing {
		d.Type = domain.MediaListing
	}
	if err := s.downloads.SaveDownload(ctx, d); err != nil {
		return fmt.Errorf("save download %s in chat %d: %w", link.URL, chatID, err)
	}

	if !media.Cached && link.Kind != domain.LinkListing && len(media.FileIDs()) > 0 {
		if err := s.downloads.SaveCached(ctx, link.ID, media); err != nil {
			return fmt.Errorf("cache media %s: %w", link.ID, err)
		}
	}
	return nil
}

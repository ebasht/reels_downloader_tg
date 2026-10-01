// Package listing fetches car sale listings from auto.ru, Avito and Drom.
package listing

import (
	"context"
	"fmt"
	"log"
	"time"

	"video_download_bot/internal/adapter/web"
	"video_download_bot/internal/domain"
)

const (
	maxPageBytes  = 5 * 1024 * 1024
	maxImageBytes = 10 * 1024 * 1024
	// Two Telegram albums, same as Instagram carousels; listings often have 30+ photos.
	maxPhotos = 20
)

// ad is a listing as parsed from its page, before photos are downloaded.
type ad struct {
	title       string
	price       string
	description string
	imageURLs   []string
}

type site struct {
	web   *web.Client
	parse func(page string) (ad, bool)
}

type Fetcher struct {
	sites map[domain.Source]site
}

func NewFetcher(timeout time.Duration) *Fetcher {
	const lang = "ru-RU,ru;q=0.9"
	return &Fetcher{sites: map[domain.Source]site{
		domain.SourceAutoRu: {
			web:   web.NewClient(timeout, web.UserAgentTelegram, lang, "auto.ru", "avto.ru", "yandex.net"),
			parse: parseAutoRu,
		},
		domain.SourceAvito: {
			web:   web.NewClient(timeout, web.UserAgentTelegram, lang, "avito.ru", "avito.st"),
			parse: parseAvito,
		},
		domain.SourceDrom: {
			web:   web.NewClient(timeout, web.UserAgentTelegram, lang, "drom.ru"),
			parse: parseDrom,
		},
	}}
}

func (f *Fetcher) FetchListing(ctx context.Context, link domain.Link) (*domain.Listing, error) {
	s, ok := f.sites[link.Source]
	if !ok {
		return nil, fmt.Errorf("unsupported listing source %q", link.Source)
	}

	resp, err := s.web.Get(ctx, link.URL, maxPageBytes)
	if err != nil {
		return nil, fmt.Errorf("load page: %w", err)
	}
	page, err := resp.Text()
	if err != nil {
		return nil, err
	}

	a, ok := s.parse(page)
	if !ok {
		return nil, fmt.Errorf("%w: no listing data on page (removed or blocked?)", domain.ErrMediaNotFound)
	}

	var images [][]byte
	for _, u := range a.imageURLs {
		if len(images) == maxPhotos {
			break
		}
		img, err := s.web.Get(ctx, u, maxImageBytes)
		if err != nil {
			log.Printf("listing photo failed: %v", err)
			continue
		}
		images = append(images, img.Body)
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("%w: no photos", domain.ErrMediaNotFound)
	}

	return &domain.Listing{
		Title:       a.title,
		Price:       a.price,
		Description: a.description,
		Images:      images,
	}, nil
}

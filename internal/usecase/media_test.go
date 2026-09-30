package usecase

import (
	"context"
	"testing"

	"video_download_bot/internal/domain"
)

type fakeReels struct{ calls []string }

func (f *fakeReels) DownloadReel(_ context.Context, url string) (*domain.Video, error) {
	f.calls = append(f.calls, url)
	return &domain.Video{Path: "video.mp4"}, nil
}

type fakePosts struct {
	post *domain.Post
	err  error
}

func (f *fakePosts) FetchPost(context.Context, string) (*domain.Post, error) {
	return f.post, f.err
}

type fakeListings struct{ calls int }

func (f *fakeListings) FetchListing(context.Context, domain.Link) (*domain.Listing, error) {
	f.calls++
	return &domain.Listing{Title: "Audi S4, 1999", Price: "1 050 000 ₽", Description: "Продаётся", Images: [][]byte{{1}, {2}}}, nil
}

type fakeDownloads struct {
	saved   []domain.Download
	cache   map[string]domain.Media
	deleted []string
}

func newFakeDownloads() *fakeDownloads {
	return &fakeDownloads{cache: map[string]domain.Media{}}
}

func (f *fakeDownloads) SaveDownload(_ context.Context, d domain.Download) error {
	f.saved = append(f.saved, d)
	return nil
}

func (f *fakeDownloads) FindCached(_ context.Context, shortcode string) (domain.Media, bool, error) {
	m, ok := f.cache[shortcode]
	return m, ok, nil
}

func (f *fakeDownloads) SaveCached(_ context.Context, shortcode string, m domain.Media) error {
	f.cache[shortcode] = m
	return nil
}

func (f *fakeDownloads) DeleteCached(_ context.Context, shortcode string) error {
	f.deleted = append(f.deleted, shortcode)
	delete(f.cache, shortcode)
	return nil
}

var (
	reelLink = domain.Link{Kind: domain.LinkReel, Source: domain.SourceInstagram, ID: "a", URL: "https://www.instagram.com/reel/a/"}
	postLink = domain.Link{Kind: domain.LinkPost, Source: domain.SourceInstagram, ID: "b", URL: "https://www.instagram.com/p/b/"}
)

func TestMediaServiceFetch(t *testing.T) {
	t.Run("reel downloads video", func(t *testing.T) {
		reels := &fakeReels{}
		m, err := NewMediaService(reels, &fakePosts{}, &fakeListings{}, newFakeDownloads()).Fetch(context.Background(), reelLink)
		if err != nil || m.Video == nil || len(reels.calls) != 1 {
			t.Fatalf("media=%+v err=%v calls=%v", m, err, reels.calls)
		}
	})

	t.Run("photo post returns post", func(t *testing.T) {
		reels := &fakeReels{}
		posts := &fakePosts{post: &domain.Post{Caption: "hi"}}
		m, err := NewMediaService(reels, posts, &fakeListings{}, newFakeDownloads()).Fetch(context.Background(), postLink)
		if err != nil || m.Post == nil || m.Post.Caption != "hi" || len(reels.calls) != 0 {
			t.Fatalf("media=%+v err=%v calls=%v", m, err, reels.calls)
		}
	})

	t.Run("video post falls back to reel downloader", func(t *testing.T) {
		reels := &fakeReels{}
		posts := &fakePosts{err: domain.ErrPostIsVideo}
		m, err := NewMediaService(reels, posts, &fakeListings{}, newFakeDownloads()).Fetch(context.Background(), postLink)
		if err != nil || m.Video == nil || len(reels.calls) != 1 || reels.calls[0] != postLink.URL {
			t.Fatalf("media=%+v err=%v calls=%v", m, err, reels.calls)
		}
	})

	t.Run("cached link is not downloaded", func(t *testing.T) {
		reels := &fakeReels{}
		downloads := newFakeDownloads()
		downloads.cache["a"] = domain.Media{Cached: true, Video: &domain.Video{FileID: "file-a"}}
		m, err := NewMediaService(reels, &fakePosts{}, &fakeListings{}, downloads).Fetch(context.Background(), reelLink)
		if err != nil || !m.Cached || len(m.FileIDs()) != 1 || m.FileIDs()[0] != "file-a" || len(reels.calls) != 0 {
			t.Fatalf("media=%+v err=%v calls=%v", m, err, reels.calls)
		}
	})

	t.Run("fetch fresh invalidates cache", func(t *testing.T) {
		reels := &fakeReels{}
		downloads := newFakeDownloads()
		downloads.cache["a"] = domain.Media{Cached: true, Video: &domain.Video{FileID: "stale"}}
		m, err := NewMediaService(reels, &fakePosts{}, &fakeListings{}, downloads).FetchFresh(context.Background(), reelLink)
		if err != nil || m.Cached || len(reels.calls) != 1 || len(downloads.cache) != 0 {
			t.Fatalf("media=%+v err=%v calls=%v cache=%v", m, err, reels.calls, downloads.cache)
		}
	})
}

func TestRecordDelivered(t *testing.T) {
	downloads := newFakeDownloads()
	svc := NewMediaService(&fakeReels{}, &fakePosts{}, &fakeListings{}, downloads)
	ctx := context.Background()

	video := domain.Media{Video: &domain.Video{FileID: "file-v"}}
	if err := svc.RecordDelivered(ctx, 42, domain.User{ID: 7}, postLink, video); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordDelivered(ctx, 42, domain.User{ID: 7}, postLink, domain.Media{Post: &domain.Post{}}); err != nil {
		t.Fatal(err)
	}

	if len(downloads.saved) != 2 || downloads.saved[0].Type != domain.MediaReel || downloads.saved[1].Type != domain.MediaPost {
		t.Fatalf("saved = %+v", downloads.saved)
	}
	if d := downloads.saved[0]; d.ChatID != 42 || d.User.ID != 7 || d.Shortcode != "b" || d.URL != postLink.URL {
		t.Fatalf("saved = %+v", d)
	}
	if got := downloads.cache["b"].FileIDs(); len(got) != 1 || got[0] != "file-v" {
		t.Fatalf("cached file ids = %q, want [file-v] (media without file ids must not overwrite them)", got)
	}
}

func TestListingIsFetchedFreshAndNotCached(t *testing.T) {
	listings := &fakeListings{}
	downloads := newFakeDownloads()
	link := domain.Link{Kind: domain.LinkListing, Source: domain.SourceDrom, ID: "323106173", URL: "https://auto.drom.ru/x/323106173.html"}
	downloads.cache[link.ID] = domain.Media{Cached: true, Post: &domain.Post{ImageFileIDs: []string{"stale"}}}
	svc := NewMediaService(&fakeReels{}, &fakePosts{}, listings, downloads)

	m, err := svc.Fetch(context.Background(), link)
	if err != nil || m.Cached || m.Post == nil || len(m.Post.Images) != 2 || listings.calls != 1 {
		t.Fatalf("media=%+v err=%v calls=%d", m, err, listings.calls)
	}
	if want := "Audi S4, 1999\n1 050 000 ₽\n\nПродаётся"; m.Post.Caption != want {
		t.Fatalf("caption = %q, want %q", m.Post.Caption, want)
	}

	delete(downloads.cache, link.ID)
	m.Post.ImageFileIDs = []string{"f1", "f2"}
	if err := svc.RecordDelivered(context.Background(), 1, domain.User{}, link, m); err != nil {
		t.Fatal(err)
	}
	if len(downloads.saved) != 1 || downloads.saved[0].Type != domain.MediaListing {
		t.Fatalf("saved = %+v", downloads.saved)
	}
	if _, cached := downloads.cache[link.ID]; cached {
		t.Fatal("listing must not be cached")
	}
}

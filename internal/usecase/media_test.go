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
	reelLink = domain.InstagramLink{Kind: domain.LinkReel, Shortcode: "a", URL: "https://www.instagram.com/reel/a/"}
	postLink = domain.InstagramLink{Kind: domain.LinkPost, Shortcode: "b", URL: "https://www.instagram.com/p/b/"}
)

func TestMediaServiceFetch(t *testing.T) {
	t.Run("reel downloads video", func(t *testing.T) {
		reels := &fakeReels{}
		m, err := NewMediaService(reels, &fakePosts{}, newFakeDownloads()).Fetch(context.Background(), reelLink)
		if err != nil || m.Video == nil || len(reels.calls) != 1 {
			t.Fatalf("media=%+v err=%v calls=%v", m, err, reels.calls)
		}
	})

	t.Run("photo post returns post", func(t *testing.T) {
		reels := &fakeReels{}
		posts := &fakePosts{post: &domain.Post{Caption: "hi"}}
		m, err := NewMediaService(reels, posts, newFakeDownloads()).Fetch(context.Background(), postLink)
		if err != nil || m.Post == nil || m.Post.Caption != "hi" || len(reels.calls) != 0 {
			t.Fatalf("media=%+v err=%v calls=%v", m, err, reels.calls)
		}
	})

	t.Run("video post falls back to reel downloader", func(t *testing.T) {
		reels := &fakeReels{}
		posts := &fakePosts{err: domain.ErrPostIsVideo}
		m, err := NewMediaService(reels, posts, newFakeDownloads()).Fetch(context.Background(), postLink)
		if err != nil || m.Video == nil || len(reels.calls) != 1 || reels.calls[0] != postLink.URL {
			t.Fatalf("media=%+v err=%v calls=%v", m, err, reels.calls)
		}
	})

	t.Run("cached link is not downloaded", func(t *testing.T) {
		reels := &fakeReels{}
		downloads := newFakeDownloads()
		downloads.cache["a"] = domain.Media{Cached: true, Video: &domain.Video{FileID: "file-a"}}
		m, err := NewMediaService(reels, &fakePosts{}, downloads).Fetch(context.Background(), reelLink)
		if err != nil || !m.Cached || m.FileID() != "file-a" || len(reels.calls) != 0 {
			t.Fatalf("media=%+v err=%v calls=%v", m, err, reels.calls)
		}
	})

	t.Run("fetch fresh invalidates cache", func(t *testing.T) {
		reels := &fakeReels{}
		downloads := newFakeDownloads()
		downloads.cache["a"] = domain.Media{Cached: true, Video: &domain.Video{FileID: "stale"}}
		m, err := NewMediaService(reels, &fakePosts{}, downloads).FetchFresh(context.Background(), reelLink)
		if err != nil || m.Cached || len(reels.calls) != 1 || len(downloads.cache) != 0 {
			t.Fatalf("media=%+v err=%v calls=%v cache=%v", m, err, reels.calls, downloads.cache)
		}
	})
}

func TestRecordDelivered(t *testing.T) {
	downloads := newFakeDownloads()
	svc := NewMediaService(&fakeReels{}, &fakePosts{}, downloads)
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
	if got := downloads.cache["b"].FileID(); got != "file-v" {
		t.Fatalf("cached file id = %q, want file-v (media without file id must not overwrite it)", got)
	}
}

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

type fakeDownloads struct{ saved []domain.Download }

func newFakeDownloads() *fakeDownloads { return &fakeDownloads{} }

func (f *fakeDownloads) SaveDownload(_ context.Context, d domain.Download) error {
	f.saved = append(f.saved, d)
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
}

func TestRecordDelivered(t *testing.T) {
	downloads := newFakeDownloads()
	svc := NewMediaService(&fakeReels{}, &fakePosts{}, &fakeListings{}, downloads)
	ctx := context.Background()

	if err := svc.RecordDelivered(ctx, 42, domain.User{ID: 7}, postLink, domain.MediaReel); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordDelivered(ctx, 42, domain.User{ID: 7}, postLink, domain.MediaPost); err != nil {
		t.Fatal(err)
	}
	if len(downloads.saved) != 2 || downloads.saved[0].Type != domain.MediaReel || downloads.saved[1].Type != domain.MediaPost {
		t.Fatalf("saved = %+v", downloads.saved)
	}
	if d := downloads.saved[0]; d.ChatID != 42 || d.User.ID != 7 || d.Shortcode != "b" || d.URL != postLink.URL {
		t.Fatalf("saved = %+v", d)
	}
}

func TestListing(t *testing.T) {
	listings := &fakeListings{}
	downloads := newFakeDownloads()
	link := domain.Link{Kind: domain.LinkListing, Source: domain.SourceDrom, ID: "323106173", URL: "https://auto.drom.ru/x/323106173.html"}
	svc := NewMediaService(&fakeReels{}, &fakePosts{}, listings, downloads)

	m, err := svc.Fetch(context.Background(), link)
	if err != nil || m.Post == nil || len(m.Post.Items) != 2 || listings.calls != 1 {
		t.Fatalf("media=%+v err=%v calls=%d", m, err, listings.calls)
	}
	if want := "Audi S4, 1999\n1 050 000 ₽\n\nПродаётся"; m.Post.Caption != want {
		t.Fatalf("caption = %q, want %q", m.Post.Caption, want)
	}

	if err := svc.RecordDelivered(context.Background(), 1, domain.User{}, link, m.Type()); err != nil {
		t.Fatal(err)
	}
	if len(downloads.saved) != 1 || downloads.saved[0].Type != domain.MediaListing {
		t.Fatalf("saved = %+v", downloads.saved)
	}
}

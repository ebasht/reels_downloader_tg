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

func TestMediaServiceFetch(t *testing.T) {
	reel := domain.InstagramLink{Kind: domain.LinkReel, URL: "https://www.instagram.com/reel/a/"}
	post := domain.InstagramLink{Kind: domain.LinkPost, URL: "https://www.instagram.com/p/b/"}

	t.Run("reel downloads video", func(t *testing.T) {
		reels := &fakeReels{}
		m, err := NewMediaService(reels, &fakePosts{}, &fakeDownloads{}).Fetch(context.Background(), reel)
		if err != nil || m.Video == nil || len(reels.calls) != 1 {
			t.Fatalf("media=%+v err=%v calls=%v", m, err, reels.calls)
		}
	})

	t.Run("photo post returns post", func(t *testing.T) {
		reels := &fakeReels{}
		posts := &fakePosts{post: &domain.Post{Caption: "hi"}}
		m, err := NewMediaService(reels, posts, &fakeDownloads{}).Fetch(context.Background(), post)
		if err != nil || m.Post == nil || m.Post.Caption != "hi" || len(reels.calls) != 0 {
			t.Fatalf("media=%+v err=%v calls=%v", m, err, reels.calls)
		}
	})

	t.Run("video post falls back to reel downloader", func(t *testing.T) {
		reels := &fakeReels{}
		posts := &fakePosts{err: domain.ErrPostIsVideo}
		m, err := NewMediaService(reels, posts, &fakeDownloads{}).Fetch(context.Background(), post)
		if err != nil || m.Video == nil || len(reels.calls) != 1 || reels.calls[0] != post.URL {
			t.Fatalf("media=%+v err=%v calls=%v", m, err, reels.calls)
		}
	})
}

type fakeDownloads struct{ saved []domain.Download }

func (f *fakeDownloads) SaveDownload(_ context.Context, d domain.Download) error {
	f.saved = append(f.saved, d)
	return nil
}

func TestRecordDeliveredUsesDeliveredType(t *testing.T) {
	downloads := &fakeDownloads{}
	svc := NewMediaService(&fakeReels{}, &fakePosts{}, downloads)
	post := domain.InstagramLink{Kind: domain.LinkPost, Shortcode: "b", URL: "https://www.instagram.com/p/b/"}

	if err := svc.RecordDelivered(context.Background(), 42, domain.User{ID: 7}, post, domain.Media{Video: &domain.Video{}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordDelivered(context.Background(), 42, domain.User{ID: 7}, post, domain.Media{Post: &domain.Post{}}); err != nil {
		t.Fatal(err)
	}

	if len(downloads.saved) != 2 || downloads.saved[0].Type != domain.MediaReel || downloads.saved[1].Type != domain.MediaPost {
		t.Fatalf("saved = %+v", downloads.saved)
	}
	if d := downloads.saved[0]; d.ChatID != 42 || d.User.ID != 7 || d.Shortcode != "b" || d.URL != post.URL {
		t.Fatalf("saved = %+v", d)
	}
}

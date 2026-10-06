//go:build integration

package instagram

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"video_download_bot/internal/domain"
)

// Run with: go test -tags integration -v ./internal/adapter/instagram/

func TestFetchPhotoPost(t *testing.T) {
	post, err := NewPostFetcher(30*time.Second).FetchPost(context.Background(), "https://www.instagram.com/p/Ddl-FKUDYTH/")
	if err != nil {
		t.Fatal(err)
	}
	// The post is a carousel of 8 photos.
	if len(post.Items) != 8 {
		t.Fatalf("got %d items, want 8", len(post.Items))
	}
	for i, it := range post.Items {
		if len(it.Data) == 0 || it.IsVideo {
			t.Fatalf("item %d: %d bytes, video=%v", i, len(it.Data), it.IsVideo)
		}
	}
	t.Logf("%d items, caption:\n%s", len(post.Items), post.Caption)
}

func TestFetchMixedCarousel(t *testing.T) {
	post, err := NewPostFetcher(60*time.Second).FetchPost(context.Background(), "https://www.instagram.com/p/DeFANL2kST5/")
	if err != nil {
		t.Fatal(err)
	}
	// 20 items: a photo, three videos, then photos.
	if len(post.Items) != 20 {
		t.Fatalf("got %d items, want 20", len(post.Items))
	}
	for i, it := range post.Items {
		if wantVideo := i >= 1 && i <= 3; it.IsVideo != wantVideo || len(it.Data) == 0 {
			t.Fatalf("item %d: %d bytes, video=%v, want video=%v", i, len(it.Data), it.IsVideo, wantVideo)
		}
	}
}

func TestFetchSinglePhotoPost(t *testing.T) {
	post, err := NewPostFetcher(30*time.Second).FetchPost(context.Background(), "https://www.instagram.com/p/DbiKF46jEF1/")
	if err != nil {
		t.Fatal(err)
	}
	if len(post.Items) != 1 || len(post.Items[0].Data) == 0 {
		t.Fatalf("got %d items, want 1 non-empty", len(post.Items))
	}
	t.Logf("image %d bytes, caption: %s", len(post.Items[0].Data), post.Caption)
}

func TestFetchPostOnReelReportsVideo(t *testing.T) {
	_, err := NewPostFetcher(30*time.Second).FetchPost(context.Background(), "https://www.instagram.com/p/Dd4BAPNsnje/")
	if !errors.Is(err, domain.ErrPostIsVideo) {
		t.Fatalf("err = %v, want ErrPostIsVideo", err)
	}
}

func TestDownloadReelRejectsLongVideo(t *testing.T) {
	_, err := NewReelDownloader(3*time.Minute, 30*time.Second).DownloadReel(context.Background(), "https://www.instagram.com/reel/Dd4BAPNsnje/")
	if !errors.Is(err, domain.ErrVideoTooLong) {
		t.Fatalf("err = %v, want ErrVideoTooLong", err)
	}
}

func TestDownloadReel(t *testing.T) {
	v, err := NewReelDownloader(3*time.Minute, 10*time.Minute).DownloadReel(context.Background(), "https://www.instagram.com/reel/Dd4BAPNsnje/")
	if err != nil {
		t.Fatal(err)
	}
	defer v.Release()

	info, err := os.Stat(v.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("video %dx%d %ds %d bytes, thumb=%q", v.Width, v.Height, v.Duration, info.Size(), v.ThumbnailPath)
}

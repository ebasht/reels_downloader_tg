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
	if len(post.Images) != 8 {
		t.Fatalf("got %d images, want 8", len(post.Images))
	}
	for i, img := range post.Images {
		if len(img) == 0 {
			t.Fatalf("image %d is empty", i)
		}
	}
	t.Logf("%d images, caption:\n%s", len(post.Images), post.Caption)
}

func TestFetchSinglePhotoPost(t *testing.T) {
	post, err := NewPostFetcher(30*time.Second).FetchPost(context.Background(), "https://www.instagram.com/p/DbiKF46jEF1/")
	if err != nil {
		t.Fatal(err)
	}
	if len(post.Images) != 1 || len(post.Images[0]) == 0 {
		t.Fatalf("got %d images, want 1 non-empty", len(post.Images))
	}
	t.Logf("image %d bytes, caption: %s", len(post.Images[0]), post.Caption)
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

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
	if len(post.Image) == 0 {
		t.Fatal("empty image")
	}
	t.Logf("image %d bytes, caption:\n%s", len(post.Image), post.Caption)
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

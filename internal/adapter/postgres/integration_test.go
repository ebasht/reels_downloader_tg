//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"video_download_bot/internal/domain"
)

// Run with: DATABASE_URL=... go test -tags integration -v ./internal/adapter/postgres/

func TestChatRepository(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	const chatID, adderID, promoterID = -100999000111, 999000111, 999000222
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM chats WHERE id = $1`, chatID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, adderID, promoterID)
	})

	repo := NewChatRepository(pool)
	chat := domain.Chat{ID: chatID, Type: "supergroup", Title: "Test chat"}
	now := time.Now().Truncate(time.Second)

	steps := []domain.MembershipChange{
		{Chat: chat, Actor: domain.User{ID: adderID, Username: "adder", FirstName: "A"}, OldStatus: domain.StatusLeft, NewStatus: domain.StatusMember, At: now},
		{Chat: chat, Actor: domain.User{ID: promoterID, FirstName: "P"}, OldStatus: domain.StatusMember, NewStatus: domain.StatusAdministrator, At: now.Add(time.Minute)},
	}
	for _, s := range steps {
		if err := repo.RecordMembership(ctx, s); err != nil {
			t.Fatal(err)
		}
	}

	var active bool
	var addedBy int64
	if err := pool.QueryRow(ctx, `SELECT is_active, added_by FROM chats WHERE id = $1`, chatID).Scan(&active, &addedBy); err != nil {
		t.Fatal(err)
	}
	if !active || addedBy != adderID {
		t.Fatalf("after promotion: active=%v added_by=%d, want true/%d", active, addedBy, adderID)
	}

	kick := domain.MembershipChange{Chat: chat, Actor: domain.User{ID: promoterID, FirstName: "P"}, OldStatus: domain.StatusAdministrator, NewStatus: domain.StatusKicked, At: now.Add(2 * time.Minute)}
	if err := repo.RecordMembership(ctx, kick); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT is_active FROM chats WHERE id = $1`, chatID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("chat still active after kick")
	}

	if err := repo.EnsureChat(ctx, chat); err != nil {
		t.Fatal(err)
	}
	var events int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM chat_membership_events WHERE chat_id = $1`, chatID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 3 {
		t.Fatalf("events = %d, want 3", events)
	}

	downloads := NewDownloadRepository(pool)
	sender := domain.User{ID: adderID, Username: "adder", FirstName: "A"}
	for _, d := range []domain.Download{
		{ChatID: chatID, User: sender, URL: "https://www.instagram.com/reel/a/", Shortcode: "a", Type: domain.MediaReel, At: now},
		{ChatID: chatID, User: sender, URL: "https://www.instagram.com/p/b/", Shortcode: "b", Type: domain.MediaReel, At: now},
		{ChatID: chatID, URL: "https://www.instagram.com/p/c/", Shortcode: "c", Type: domain.MediaPost, At: now},
	} {
		if err := downloads.SaveDownload(ctx, d); err != nil {
			t.Fatal(err)
		}
	}

	var reels, posts, rows int
	if err := pool.QueryRow(ctx, `SELECT reels_downloaded, posts_downloaded FROM chats WHERE id = $1`, chatID).Scan(&reels, &posts); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM downloads WHERE chat_id = $1`, chatID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if reels != 2 || posts != 1 || rows != 3 {
		t.Fatalf("reels=%d posts=%d rows=%d, want 2/1/3", reels, posts, rows)
	}

	const shortcode = "integration-test-shortcode"
	t.Cleanup(func() { _ = downloads.DeleteCached(ctx, shortcode) })
	video := domain.Media{Video: &domain.Video{FileID: "file-1", Width: 720, Height: 1280, Duration: 59}}
	if err := downloads.SaveCached(ctx, shortcode, video); err != nil {
		t.Fatal(err)
	}
	cached, ok, err := downloads.FindCached(ctx, shortcode)
	if err != nil || !ok || !cached.Cached || cached.Video == nil ||
		cached.Video.FileID != "file-1" || cached.Video.Width != 720 || cached.Video.Height != 1280 || cached.Video.Duration != 59 {
		t.Fatalf("cached=%+v ok=%v err=%v", cached, ok, err)
	}
	if err := downloads.DeleteCached(ctx, shortcode); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := downloads.FindCached(ctx, shortcode); ok {
		t.Fatal("cache entry not deleted")
	}
}

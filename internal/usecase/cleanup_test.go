package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"video_download_bot/internal/domain"
)

type fakeDeletions struct {
	items map[[2]int64]time.Time
}

func newFakeDeletions() *fakeDeletions {
	return &fakeDeletions{items: map[[2]int64]time.Time{}}
}

func (f *fakeDeletions) ScheduleDeletion(_ context.Context, chatID int64, ids []int, at time.Time) error {
	for _, id := range ids {
		f.items[[2]int64{chatID, int64(id)}] = at
	}
	return nil
}

func (f *fakeDeletions) DueDeletions(_ context.Context, now time.Time, _ int) ([]domain.ScheduledDeletion, error) {
	var due []domain.ScheduledDeletion
	for k, at := range f.items {
		if !at.After(now) {
			due = append(due, domain.ScheduledDeletion{ChatID: k[0], MessageID: int(k[1]), At: at})
		}
	}
	return due, nil
}

func (f *fakeDeletions) RemoveDeletion(_ context.Context, chatID int64, id int) error {
	delete(f.items, [2]int64{chatID, int64(id)})
	return nil
}

type fakeDeleter struct {
	errs    map[int]error
	deleted []int
}

func (f *fakeDeleter) DeleteMessage(_ int64, id int) error {
	if err := f.errs[id]; err != nil {
		return err
	}
	f.deleted = append(f.deleted, id)
	return nil
}

func TestCleanupSchedulesAfterTTL(t *testing.T) {
	repo := newFakeDeletions()
	svc := NewCleanupService(repo, 10*time.Hour)
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	if err := svc.Schedule(context.Background(), -100, []int{1, 2}); err != nil {
		t.Fatal(err)
	}
	if at := repo.items[[2]int64{-100, 2}]; !at.Equal(now.Add(10 * time.Hour)) {
		t.Fatalf("delete at %v, want %v", at, now.Add(10*time.Hour))
	}

	disabled := NewCleanupService(newFakeDeletions(), 0)
	if err := disabled.Schedule(context.Background(), -100, []int{1}); err != nil || len(disabled.repo.(*fakeDeletions).items) != 0 {
		t.Fatalf("disabled cleanup scheduled messages: err=%v", err)
	}
}

func TestCleanupDeleteDue(t *testing.T) {
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	repo := newFakeDeletions()
	repo.items[[2]int64{1, 1}] = now.Add(-time.Minute)   // deleted
	repo.items[[2]int64{1, 2}] = now.Add(-time.Minute)   // already gone: dropped
	repo.items[[2]int64{1, 3}] = now.Add(-time.Minute)   // transient error: retried later
	repo.items[[2]int64{1, 4}] = now.Add(-7 * time.Hour) // failing for too long: dropped
	repo.items[[2]int64{1, 5}] = now.Add(time.Hour)      // not due yet

	transient := errors.New("Bad Gateway")
	deleter := &fakeDeleter{errs: map[int]error{
		2: fmt.Errorf("%w: message to delete not found", domain.ErrMessageGone),
		3: transient,
		4: transient,
	}}
	svc := NewCleanupService(repo, 10*time.Hour)
	svc.now = func() time.Time { return now }

	svc.deleteDue(context.Background(), deleter)

	if len(deleter.deleted) != 1 || deleter.deleted[0] != 1 {
		t.Fatalf("deleted %v, want [1]", deleter.deleted)
	}
	for id, wantLeft := range map[int64]bool{1: false, 2: false, 3: true, 4: false, 5: true} {
		if _, left := repo.items[[2]int64{1, id}]; left != wantLeft {
			t.Errorf("message %d still scheduled = %v, want %v", id, left, wantLeft)
		}
	}
}

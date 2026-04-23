package core

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"reminder-bot/internal/storage"
)

func TestProcessDueReminders_NotifiesAndStoresMessageID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := newMockFullRepo()
	manager := NewNotificationManager(repo, logger)
	ctx := context.Background()
	notifier := &mockNotifier{}

	rem := &storage.Reminder{ChatID: 1, Text: "Due now", NotifyAt: time.Now().Add(-1 * time.Minute)}
	_ = repo.Add(ctx, rem)

	manager.ProcessDueReminders(ctx, notifier)

	if len(notifier.notified) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.notified))
	}
	if notifier.notified[0].Text != "Due now" {
		t.Errorf("wrong reminder notified: %q", notifier.notified[0].Text)
	}

	// Verify LastMessageID was stored
	updated, _ := repo.GetByID(ctx, rem.ID)
	if updated.LastMessageID != 999 {
		t.Errorf("expected LastMessageID=999, got %d", updated.LastMessageID)
	}
}

func TestProcessDueReminders_DeletesOldMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := newMockFullRepo()
	manager := NewNotificationManager(repo, logger)
	ctx := context.Background()
	notifier := &mockNotifier{}

	rem := &storage.Reminder{
		ChatID: 1, Text: "Repeat", NotifyAt: time.Now().Add(-1 * time.Minute),
		LastMessageID: 42,
	}
	_ = repo.Add(ctx, rem)

	manager.ProcessDueReminders(ctx, notifier)

	if len(notifier.deleted) != 1 || notifier.deleted[0] != 42 {
		t.Errorf("expected old message 42 to be deleted, got %v", notifier.deleted)
	}
}

func TestProcessDueReminders_RetryOnNotifyFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := newMockFullRepo()
	manager := NewNotificationManager(repo, logger)
	ctx := context.Background()
	notifier := &mockNotifier{failOnce: true}

	now := time.Now()
	rem := &storage.Reminder{ChatID: 1, Text: "Retry Me", NotifyAt: now.Add(-1 * time.Minute)}
	_ = repo.Add(ctx, rem)

	// First attempt: Notify fails, reminder rescheduled ~1min ahead
	manager.ProcessDueReminders(ctx, notifier)
	if len(notifier.notified) != 0 {
		t.Error("expected no notification due to failure")
	}
	updated, _ := repo.GetByID(ctx, rem.ID)
	if !updated.NotifyAt.After(now) {
		t.Error("expected reminder time to be moved forward for retry")
	}

	// Move it back to due
	updated.NotifyAt = now.Add(-1 * time.Minute)
	_ = repo.Update(ctx, updated)

	// Second attempt: should succeed
	manager.ProcessDueReminders(ctx, notifier)
	if len(notifier.notified) != 1 {
		t.Error("expected 1 notification on second attempt")
	}
}

func TestProcessDueReminders_SkipsNonDue(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := newMockFullRepo()
	manager := NewNotificationManager(repo, logger)
	ctx := context.Background()
	notifier := &mockNotifier{}

	rem := &storage.Reminder{ChatID: 1, Text: "Future", NotifyAt: time.Now().Add(1 * time.Hour)}
	_ = repo.Add(ctx, rem)
	manager.ProcessDueReminders(ctx, notifier)

	if len(notifier.notified) != 0 {
		t.Error("future reminder should not be notified")
	}
}

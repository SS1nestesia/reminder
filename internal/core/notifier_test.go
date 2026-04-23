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

// TestProcessDueReminders_AdvancesNotifyAtAfterSuccess is a regression test for
// a bug where Update(r) after a successful Notify overwrote notify_at back to
// the past (original due time), causing duplicate notifications on every
// scheduler tick. The fix: sync r.NotifyAt to the back-off time written by
// MarkAsNotified BEFORE issuing Update.
func TestProcessDueReminders_AdvancesNotifyAtAfterSuccess(t *testing.T) {
        logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
        repo := newMockFullRepo()
        manager := NewNotificationManager(repo, logger)
        ctx := context.Background()
        notifier := &mockNotifier{}

        originalDue := time.Now().Add(-5 * time.Minute).UTC()
        rem := &storage.Reminder{ChatID: 1, Text: "Due since 5min", NotifyAt: originalDue}
        _ = repo.Add(ctx, rem)

        manager.ProcessDueReminders(ctx, notifier)

        updated, _ := repo.GetByID(ctx, rem.ID)
        // After a successful notification, notify_at must be moved forward
        // (to the back-off time). It must NOT equal the original past time.
        if updated.NotifyAt.Equal(originalDue) {
                t.Fatal("notify_at regressed to the original past due time — reminder would fire again immediately")
        }
        if !updated.NotifyAt.After(time.Now()) {
                t.Errorf("expected notify_at to be in the future (back-off), got %v (now=%v)", updated.NotifyAt, time.Now())
        }
        if updated.LastMessageID != 999 {
                t.Errorf("LastMessageID should still be stored, got %d", updated.LastMessageID)
        }
}

// TestProcessDueReminders_NoDuplicateFireOnConsecutiveTicks verifies the
// end-to-end effect of the back-off: a reminder that just fired must NOT
// appear in the next GetDue() result.
func TestProcessDueReminders_NoDuplicateFireOnConsecutiveTicks(t *testing.T) {
        logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
        repo := newMockFullRepo()
        manager := NewNotificationManager(repo, logger)
        ctx := context.Background()
        notifier := &mockNotifier{}

        rem := &storage.Reminder{ChatID: 1, Text: "Only once", NotifyAt: time.Now().Add(-1 * time.Minute)}
        _ = repo.Add(ctx, rem)

        // First tick — should notify once
        manager.ProcessDueReminders(ctx, notifier)
        if len(notifier.notified) != 1 {
                t.Fatalf("expected 1 notification on first tick, got %d", len(notifier.notified))
        }

        // Simulate the very next scheduler tick (a few seconds later). The
        // reminder must NOT be due yet due to the 1-minute back-off.
        manager.ProcessDueReminders(ctx, notifier)
        if len(notifier.notified) != 1 {
                t.Errorf("expected NO additional notification on second tick (back-off should protect), got total %d", len(notifier.notified))
        }
}


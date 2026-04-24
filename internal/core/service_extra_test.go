package core

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"reminder-bot/internal/storage"
)

// ==========================================
// Snooze chain — PROJECT_OVERVIEW Section D case 16
// ==========================================

// TestSnooze_Chain_RelativeToLastTrigger verifies that each snooze shifts
// notify_at relative to the latest trigger, not to the original one.
func TestSnooze_Chain_RelativeToLastTrigger(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()
	chatID := int64(1)

	start := time.Now().UTC().Add(-30 * time.Minute)
	id, _ := s.AddReminder(ctx, chatID, "Take pills", start)

	// 1st snooze: +5m from now
	if err := s.SnoozeReminder(ctx, chatID, id, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	after1, _ := s.GetReminder(ctx, id)
	want1 := time.Now().UTC().Add(5 * time.Minute)
	if diff := after1.NotifyAt.Sub(want1); diff < -time.Second || diff > time.Second {
		t.Fatalf("1st snooze: got %v, want ~%v", after1.NotifyAt, want1)
	}

	// Simulate the reminder firing again; user snoozes a second time by +1h
	if err := s.SnoozeReminder(ctx, chatID, id, time.Hour); err != nil {
		t.Fatal(err)
	}
	after2, _ := s.GetReminder(ctx, id)
	want2 := time.Now().UTC().Add(time.Hour)
	if diff := after2.NotifyAt.Sub(want2); diff < -time.Second || diff > time.Second {
		t.Errorf("2nd snooze: got %v, want ~%v (must be relative to NOW, not original start)", after2.NotifyAt, want2)
	}

	// LastMessageID must be reset after each snooze so the scheduler resends
	if after2.LastMessageID != 0 {
		t.Errorf("LastMessageID should be 0 after snooze, got %d", after2.LastMessageID)
	}
}

// ==========================================
// Weekly bitmask invariant — PROJECT_OVERVIEW Section B case 5
// ==========================================

// TestUpdateReminderWeekdays_BitmaskValueMonWedFri verifies the documented
// bitmask convention: Mon=1, Wed=4, Fri=16 → combined = 21.
func TestUpdateReminderWeekdays_BitmaskValueMonWedFri(t *testing.T) {
	s, repo, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminder(ctx, 1, "Gym", time.Now().UTC())
	const monWedFri = (1 << 0) | (1 << 2) | (1 << 4) // 21

	if err := s.UpdateReminderWeekdays(ctx, 1, id, monWedFri); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(ctx, id)
	if got.Weekdays != 21 {
		t.Errorf("expected weekdays bitmask 21 (Mon+Wed+Fri), got %d", got.Weekdays)
	}
}

// ==========================================
// AddReminderForFriend carries author & shares ownership
// ==========================================

func TestAddReminderForFriend_StoresAuthorAndTarget(t *testing.T) {
	s, repo, _ := newTestService()
	ctx := context.Background()

	id, err := s.AddReminderForFriend(ctx, 100 /* author */, 200 /* target */, "Drink water", time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	got, _ := repo.GetByID(ctx, id)
	if got.ChatID != 200 {
		t.Errorf("ChatID = %d, want 200 (target)", got.ChatID)
	}
	if got.AuthorID != 100 {
		t.Errorf("AuthorID = %d, want 100 (author)", got.AuthorID)
	}
}

func TestDeleteFriendReminder_AuthorCanDeleteTargetOwnership(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminderForFriend(ctx, 100, 200, "Shared", time.Now().UTC())

	// Author can delete even though chat_id != author
	r, err := s.DeleteFriendReminder(ctx, 100, id)
	if err != nil {
		t.Fatalf("author should be allowed to delete, got %v", err)
	}
	if r == nil {
		t.Fatal("expected deleted reminder to be returned")
	}
	if r.AuthorID != 100 || r.ChatID != 200 {
		t.Errorf("unexpected returned reminder fields: %+v", r)
	}
}

func TestDeleteFriendReminder_TargetCanDelete(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminderForFriend(ctx, 100, 200, "Shared", time.Now().UTC())
	// Target (owner) also allowed
	if _, err := s.DeleteFriendReminder(ctx, 200, id); err != nil {
		t.Errorf("target should be allowed to delete, got %v", err)
	}
}

func TestDeleteFriendReminder_OtherUser_Forbidden(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminderForFriend(ctx, 100, 200, "Shared", time.Now().UTC())
	if _, err := s.DeleteFriendReminder(ctx, 999, id); err == nil {
		t.Error("unrelated user must NOT be able to delete shared reminder")
	}
}

func TestUpdateFriendReminderTime_AuthorCanReschedule(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminderForFriend(ctx, 100, 200, "X", time.Now().UTC())
	newTime := time.Now().UTC().Add(48 * time.Hour)
	r, err := s.UpdateFriendReminderTime(ctx, 100, id, newTime)
	if err != nil {
		t.Fatalf("author should reschedule: %v", err)
	}
	if !r.NotifyAt.Equal(newTime) {
		t.Errorf("NotifyAt = %v, want %v", r.NotifyAt, newTime)
	}
}

// ==========================================
// Batch scheduling — PROJECT_OVERVIEW Section D case 13
// ==========================================

// TestProcessDueReminders_LargeBatch_NoneSkipped verifies that when many
// reminders are due in a single scheduler tick, every one of them is
// notified in the same batch. The scheduler itself runs serially, so
// concurrency here is an invariant of the LOOP (not of parallel calls).
func TestProcessDueReminders_LargeBatch_NoneSkipped(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(discardWriter{}, nil))
	repo := newMockFullRepo()
	manager := NewNotificationManager(repo, logger)
	ctx := context.Background()

	const N = 50
	due := time.Now().Add(-time.Minute)
	for i := 0; i < N; i++ {
		_ = repo.Add(ctx, &storage.Reminder{
			ChatID: int64(i + 1), Text: "batch", NotifyAt: due,
		})
	}

	var counter int32
	n := &countingNotifier{inc: &counter}
	manager.ProcessDueReminders(ctx, n)

	if got := atomic.LoadInt32(&counter); got != N {
		t.Errorf("expected %d notifications in one batch, got %d", N, got)
	}

	// After the pass, none of the reminders should still be due
	// (all must have the 1-minute back-off applied).
	left, _ := repo.GetDue(ctx, time.Now())
	if len(left) != 0 {
		t.Errorf("expected no due reminders after batch, got %d", len(left))
	}
}

type countingNotifier struct {
	inc *int32
	mu  sync.Mutex
}

func (c *countingNotifier) Notify(_ context.Context, _ storage.Reminder) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	atomic.AddInt32(c.inc, 1)
	return 1, nil
}
func (c *countingNotifier) DeleteMessage(_ context.Context, _ int64, _ int) error { return nil }

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// ==========================================
// Timezone Shift
// ==========================================

func TestUpdateTimezoneForReminders(t *testing.T) {
	s, repo, sess := newTestService()
	ctx := context.Background()

	// Add reminder in UTC+3 (Europe/Moscow)
	locMoscow, _ := time.LoadLocation("Europe/Moscow")
	notifyAtMoscow := time.Date(2026, 4, 24, 10, 0, 0, 0, locMoscow)
	id, _ := s.AddReminder(ctx, 1, "TZ Test", notifyAtMoscow.UTC())

	_ = sess.SetTimezone(ctx, 1, "Asia/Dubai")

	err := s.UpdateTimezoneForReminders(ctx, 1, "Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}

	r, _ := repo.GetByID(ctx, id)
	
	// The wall clock time should remain 10:00 but in Asia/Dubai.
	locDubai, _ := time.LoadLocation("Asia/Dubai")
	expectedDubai := time.Date(2026, 4, 24, 10, 0, 0, 0, locDubai).UTC()

	if !r.NotifyAt.Equal(expectedDubai) {
		t.Errorf("NotifyAt = %v, want %v", r.NotifyAt, expectedDubai)
	}
}

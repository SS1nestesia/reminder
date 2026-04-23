package storage

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func newTestStorage(t *testing.T) (Storage, func()) {
	t.Helper()
	dbPath := "test_" + t.Name() + ".db"
	s, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	return s, func() {
		s.Close()
		os.Remove(dbPath)
	}
}

// ==========================================
// Reminders CRUD
// ==========================================

func TestReminders_AddAndGetByID(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	repo := s.Reminders()
	ctx := context.Background()

	now := time.Now().Truncate(time.Second).UTC()
	rem := &Reminder{ChatID: 1, Text: "Test", NotifyAt: now, Interval: "24h", Weekdays: 3}
	if err := repo.Add(ctx, rem); err != nil {
		t.Fatal(err)
	}
	if rem.ID == 0 {
		t.Error("expected non-zero ID after Add")
	}

	got, err := repo.GetByID(ctx, rem.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "Test" || got.ChatID != 1 || got.Interval != "24h" || got.Weekdays != 3 {
		t.Errorf("unexpected reminder data: %+v", got)
	}
	if !got.NotifyAt.Equal(now) {
		t.Errorf("expected NotifyAt=%v, got %v", now, got.NotifyAt)
	}
}

func TestReminders_GetByID_NotFound(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()

	_, err := s.Reminders().GetByID(context.Background(), 999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestReminders_Update(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	repo := s.Reminders()
	ctx := context.Background()

	rem := &Reminder{ChatID: 1, Text: "Original", NotifyAt: time.Now()}
	_ = repo.Add(ctx, rem)

	rem.Text = "Updated"
	rem.Interval = "48h"
	if err := repo.Update(ctx, rem); err != nil {
		t.Fatal(err)
	}

	got, _ := repo.GetByID(ctx, rem.ID)
	if got.Text != "Updated" || got.Interval != "48h" {
		t.Errorf("update failed: text=%q interval=%q", got.Text, got.Interval)
	}
}

func TestReminders_Delete(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	repo := s.Reminders()
	ctx := context.Background()

	rem := &Reminder{ChatID: 1, Text: "Delete me", NotifyAt: time.Now()}
	_ = repo.Add(ctx, rem)

	if err := repo.Delete(ctx, 1, rem.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, rem.ID); !errors.Is(err, ErrNotFound) {
		t.Error("reminder should be deleted")
	}
}

func TestReminders_Delete_WrongChatID(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	repo := s.Reminders()
	ctx := context.Background()

	rem := &Reminder{ChatID: 1, Text: "Not yours", NotifyAt: time.Now()}
	_ = repo.Add(ctx, rem)

	if err := repo.Delete(ctx, 2, rem.ID); !errors.Is(err, ErrNotFound) {
		t.Error("should not delete reminder belonging to another chat")
	}
}

func TestReminders_Delete_NonExistent(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()

	err := s.Reminders().Delete(context.Background(), 1, 999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestReminders_DeleteByID(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	repo := s.Reminders()
	ctx := context.Background()

	rem := &Reminder{ChatID: 1, Text: "Test", NotifyAt: time.Now()}
	_ = repo.Add(ctx, rem)

	if err := repo.DeleteByID(ctx, rem.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, rem.ID); !errors.Is(err, ErrNotFound) {
		t.Error("reminder should be deleted")
	}
}

func TestReminders_DeleteByID_NonExistent(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()

	err := s.Reminders().DeleteByID(context.Background(), 999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ==========================================
// GetByChatID — ordering and isolation
// ==========================================

func TestReminders_GetByChatID_OrderedByTime(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	repo := s.Reminders()
	ctx := context.Background()

	now := time.Now()
	_ = repo.Add(ctx, &Reminder{ChatID: 1, Text: "Later", NotifyAt: now.Add(2 * time.Hour)})
	_ = repo.Add(ctx, &Reminder{ChatID: 1, Text: "Sooner", NotifyAt: now.Add(1 * time.Hour)})
	_ = repo.Add(ctx, &Reminder{ChatID: 2, Text: "Other", NotifyAt: now}) // different chat

	rems, err := repo.GetByChatID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rems) != 2 {
		t.Fatalf("expected 2 reminders for chat 1, got %d", len(rems))
	}
	if rems[0].Text != "Sooner" || rems[1].Text != "Later" {
		t.Error("reminders not ordered by notify_at ASC")
	}
}

func TestReminders_GetByChatID_Empty(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()

	rems, err := s.Reminders().GetByChatID(context.Background(), 999)
	if err != nil {
		t.Fatal(err)
	}
	if len(rems) != 0 {
		t.Errorf("expected empty list, got %d", len(rems))
	}
}

// ==========================================
// GetDue — time filtering
// ==========================================

func TestReminders_GetDue_FiltersCorrectly(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	repo := s.Reminders()
	ctx := context.Background()

	now := time.Now().UTC()
	_ = repo.Add(ctx, &Reminder{ChatID: 1, Text: "Past", NotifyAt: now.Add(-10 * time.Minute)})
	_ = repo.Add(ctx, &Reminder{ChatID: 1, Text: "Future", NotifyAt: now.Add(10 * time.Minute)})

	due, err := repo.GetDue(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 {
		t.Fatalf("expected 1 due reminder, got %d", len(due))
	}
	if due[0].Text != "Past" {
		t.Errorf("expected 'Past', got %q", due[0].Text)
	}
}

func TestReminders_GetDue_Empty(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()

	due, err := s.Reminders().GetDue(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Errorf("expected no due reminders, got %d", len(due))
	}
}

// ==========================================
// MarkAsNotified
// ==========================================

func TestReminders_MarkAsNotified(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	repo := s.Reminders()
	ctx := context.Background()

	rem := &Reminder{ChatID: 1, Text: "Notify me", NotifyAt: time.Now().Add(-1 * time.Minute)}
	_ = repo.Add(ctx, rem)

	newTime := time.Now().Add(1 * time.Minute).Truncate(time.Second).UTC()
	if err := repo.MarkAsNotified(ctx, rem.ID, newTime); err != nil {
		t.Fatal(err)
	}

	updated, _ := repo.GetByID(ctx, rem.ID)
	if !updated.NotifyAt.Equal(newTime) {
		t.Errorf("MarkAsNotified: expected %v, got %v", newTime, updated.NotifyAt)
	}
}

func TestReminders_MarkAsNotified_NonExistent(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()

	err := s.Reminders().MarkAsNotified(context.Background(), 999, time.Now())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ==========================================
// Sessions
// ==========================================

func TestSessions_StateLifecycle(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	sess := s.Sessions()
	ctx := context.Background()
	chatID := int64(42)

	// Initially empty
	state, _ := sess.GetState(ctx, chatID)
	if state != "" {
		t.Errorf("expected empty state, got %q", state)
	}

	// Set and get
	_ = sess.SetState(ctx, chatID, "state1")
	state, _ = sess.GetState(ctx, chatID)
	if state != "state1" {
		t.Errorf("expected 'state1', got %q", state)
	}

	// Upsert
	_ = sess.SetState(ctx, chatID, "state2")
	state, _ = sess.GetState(ctx, chatID)
	if state != "state2" {
		t.Errorf("expected 'state2', got %q", state)
	}

	// Delete
	_ = sess.DeleteState(ctx, chatID)
	state, _ = sess.GetState(ctx, chatID)
	if state != "" {
		t.Error("expected state to be deleted")
	}
}

func TestSessions_PendingText(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	sess := s.Sessions()
	ctx := context.Background()

	_ = sess.SetPendingText(ctx, 1, "buy milk")
	text, _ := sess.GetPendingText(ctx, 1)
	if text != "buy milk" {
		t.Errorf("expected 'buy milk', got %q", text)
	}

	_ = sess.ClearPendingText(ctx, 1)
	text, _ = sess.GetPendingText(ctx, 1)
	if text != "" {
		t.Error("pending text should be cleared")
	}
}

func TestSessions_PendingReminderID(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	sess := s.Sessions()
	ctx := context.Background()

	_ = sess.SetPendingReminderID(ctx, 1, 42)
	id, _ := sess.GetPendingReminderID(ctx, 1)
	if id != 42 {
		t.Errorf("expected 42, got %d", id)
	}
}

func TestSessions_SessionMessageID(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	sess := s.Sessions()
	ctx := context.Background()

	_ = sess.SetSessionMessageID(ctx, 1, 123)
	msgID, _ := sess.GetSessionMessageID(ctx, 1)
	if msgID != 123 {
		t.Errorf("expected 123, got %d", msgID)
	}
}

func TestSessions_FieldsIndependent(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	sess := s.Sessions()
	ctx := context.Background()
	chatID := int64(1)

	// Set state first
	_ = sess.SetState(ctx, chatID, "editing:5")

	// Set pending text — should not overwrite state
	_ = sess.SetPendingText(ctx, chatID, "some text")
	state, _ := sess.GetState(ctx, chatID)
	if state != "editing:5" {
		t.Errorf("SetPendingText should not change state, got %q", state)
	}

	// Set pending ID — should not overwrite state
	_ = sess.SetPendingReminderID(ctx, chatID, 99)
	state, _ = sess.GetState(ctx, chatID)
	if state != "editing:5" {
		t.Errorf("SetPendingReminderID should not change state, got %q", state)
	}
}

// TestSessions_SetFieldOnFreshChat verifies that calling Set*ID/Set*Text
// on a brand new chatID (no prior SetState) does not produce a session
// with corrupted state. This is a regression test for the upsert bug.
func TestSessions_SetFieldOnFreshChat(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	sess := s.Sessions()
	ctx := context.Background()
	freshChat := int64(999)

	// Set pending reminder ID without prior SetState
	_ = sess.SetPendingReminderID(ctx, freshChat, 42)

	// State should be empty (default), NOT clobbered
	state, _ := sess.GetState(ctx, freshChat)
	if state != "" {
		t.Errorf("expected empty state for fresh chat, got %q", state)
	}

	// The pending ID should still work
	id, _ := sess.GetPendingReminderID(ctx, freshChat)
	if id != 42 {
		t.Errorf("expected pending ID 42, got %d", id)
	}

	// Now set state — pending ID should be preserved
	_ = sess.SetState(ctx, freshChat, "some_state")
	id, _ = sess.GetPendingReminderID(ctx, freshChat)
	if id != 42 {
		t.Errorf("SetState clobbered pending ID, got %d", id)
	}
}

// ==========================================
// Cleanup
// ==========================================

func TestSessions_Cleanup(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	sess := s.Sessions()
	ctx := context.Background()

	db := s.(*sqliteStorage).db

	old := time.Now().Add(-48 * time.Hour)
	recent := time.Now().Add(-1 * time.Hour)

	_, _ = db.Exec("INSERT INTO user_states (chat_id, state, updated_at) VALUES (?, ?, ?)", 101, "old", old.Format("2006-01-02 15:04:05"))
	_, _ = db.Exec("INSERT INTO user_states (chat_id, state, updated_at) VALUES (?, ?, ?)", 102, "recent", recent.Format("2006-01-02 15:04:05"))

	cutoff := time.Now().Add(-24 * time.Hour)
	if err := sess.Cleanup(ctx, cutoff); err != nil {
		t.Fatal(err)
	}

	// Old session should be deleted
	state, _ := sess.GetState(ctx, 101)
	if state != "" {
		t.Error("old session should be cleaned up")
	}

	// Recent session should remain
	state, _ = sess.GetState(ctx, 102)
	if state != "recent" {
		t.Errorf("recent session should remain, got %q", state)
	}
}

func TestSessions_Timezone(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	sess := s.Sessions()
	ctx := context.Background()

	_ = sess.SetTimezone(ctx, 1, "Europe/Moscow")
	tz, _ := sess.GetTimezone(ctx, 1)
	if tz != "Europe/Moscow" {
		t.Errorf("expected 'Europe/Moscow', got %q", tz)
	}

	tz2, _ := sess.GetTimezone(ctx, 999)
	if tz2 != "" {
		t.Errorf("expected empty string, got %q", tz2)
	}
}

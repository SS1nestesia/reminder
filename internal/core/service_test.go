package core

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"reminder-bot/internal/storage"
)

// --- Mocks ---

type mockFullRepo struct {
	storage.ReminderRepository
	reminders map[int64]storage.Reminder
	nextID    int64
}

func newMockFullRepo() *mockFullRepo {
	return &mockFullRepo{
		reminders: make(map[int64]storage.Reminder),
		nextID:    1,
	}
}

func (m *mockFullRepo) Add(ctx context.Context, r *storage.Reminder) error {
	r.ID = m.nextID
	m.nextID++
	m.reminders[r.ID] = *r
	return nil
}

func (m *mockFullRepo) GetByID(ctx context.Context, id int64) (*storage.Reminder, error) {
	r, ok := m.reminders[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &r, nil
}

func (m *mockFullRepo) Update(ctx context.Context, r *storage.Reminder) error {
	if _, ok := m.reminders[r.ID]; !ok {
		return storage.ErrNotFound
	}
	m.reminders[r.ID] = *r
	return nil
}

func (m *mockFullRepo) Delete(ctx context.Context, chatID, id int64) error {
	r, ok := m.reminders[id]
	if !ok || r.ChatID != chatID {
		return storage.ErrNotFound
	}
	delete(m.reminders, id)
	return nil
}

func (m *mockFullRepo) GetByChatID(ctx context.Context, chatID int64) ([]storage.Reminder, error) {
	var list []storage.Reminder
	for _, r := range m.reminders {
		if r.ChatID == chatID {
			list = append(list, r)
		}
	}
	return list, nil
}

func (m *mockFullRepo) GetDue(ctx context.Context, before time.Time) ([]storage.Reminder, error) {
	var due []storage.Reminder
	for _, r := range m.reminders {
		if r.NotifyAt.Before(before) || r.NotifyAt.Equal(before) {
			due = append(due, r)
		}
	}
	return due, nil
}

func (m *mockFullRepo) MarkAsNotified(ctx context.Context, id int64, nextNotifyAt time.Time) error {
	r, ok := m.reminders[id]
	if !ok {
		return storage.ErrNotFound
	}
	r.NotifyAt = nextNotifyAt
	m.reminders[id] = r
	return nil
}

func (m *mockFullRepo) DeleteByID(ctx context.Context, id int64) error {
	if _, ok := m.reminders[id]; !ok {
		return storage.ErrNotFound
	}
	delete(m.reminders, id)
	return nil
}

func (m *mockFullRepo) GetByAuthorAndTarget(ctx context.Context, authorID, targetChatID int64) ([]storage.Reminder, error) {
	var list []storage.Reminder
	for _, r := range m.reminders {
		if r.AuthorID == authorID && r.ChatID == targetChatID {
			list = append(list, r)
		}
	}
	return list, nil
}

func (m *mockFullRepo) GetFriendReminders(ctx context.Context, chatID int64) ([]storage.Reminder, error) {
	var list []storage.Reminder
	for _, r := range m.reminders {
		if r.ChatID == chatID && r.AuthorID != 0 {
			list = append(list, r)
		}
	}
	return list, nil
}

func (m *mockFullRepo) ClearAuthor(ctx context.Context, authorID, targetChatID int64) error {
	for id, r := range m.reminders {
		if r.AuthorID == authorID && r.ChatID == targetChatID {
			r.AuthorID = 0
			m.reminders[id] = r
		}
	}
	return nil
}

type mockSessionRepo struct {
	storage.SessionRepository
	cleaned      bool
	pendingID    int64
	state        string
	pendingText  string
	sessionMsgID int
	timezone     string
}

func (m *mockSessionRepo) Cleanup(ctx context.Context, olderThan time.Time) error {
	m.cleaned = true
	return nil
}

func (m *mockSessionRepo) SetPendingReminderID(ctx context.Context, chatID, id int64) error {
	m.pendingID = id
	return nil
}

func (m *mockSessionRepo) GetPendingReminderID(ctx context.Context, chatID int64) (int64, error) {
	return m.pendingID, nil
}

func (m *mockSessionRepo) SetState(ctx context.Context, chatID int64, state string) error {
	m.state = state
	return nil
}

func (m *mockSessionRepo) GetState(ctx context.Context, chatID int64) (string, error) {
	return m.state, nil
}

func (m *mockSessionRepo) DeleteState(ctx context.Context, chatID int64) error {
	m.state = ""
	return nil
}

func (m *mockSessionRepo) SetPendingText(ctx context.Context, chatID int64, text string) error {
	m.pendingText = text
	return nil
}

func (m *mockSessionRepo) GetPendingText(ctx context.Context, chatID int64) (string, error) {
	return m.pendingText, nil
}

func (m *mockSessionRepo) ClearPendingText(ctx context.Context, chatID int64) error {
	m.pendingText = ""
	return nil
}

func (m *mockSessionRepo) SetTimezone(ctx context.Context, chatID int64, tz string) error {
	m.timezone = tz
	return nil
}

func (m *mockSessionRepo) GetTimezone(ctx context.Context, chatID int64) (string, error) {
	return m.timezone, nil
}

func (m *mockSessionRepo) SetSessionMessageID(ctx context.Context, chatID int64, msgID int) error {
	m.sessionMsgID = msgID
	return nil
}

func (m *mockSessionRepo) GetSessionMessageID(ctx context.Context, chatID int64) (int, error) {
	return m.sessionMsgID, nil
}

type mockNotifier struct {
	notified  []storage.Reminder
	deleted   []int
	failOnce  bool
	failCount int
}

func (m *mockNotifier) Notify(ctx context.Context, r storage.Reminder) (int, error) {
	if m.failOnce && m.failCount == 0 {
		m.failCount++
		return 0, errors.New("temporary failure")
	}
	m.notified = append(m.notified, r)
	return 999, nil
}

func (m *mockNotifier) DeleteMessage(ctx context.Context, chatID int64, msgID int) error {
	m.deleted = append(m.deleted, msgID)
	return nil
}

// --- Helper ---

func newTestService() (*ReminderService, *mockFullRepo, *mockSessionRepo) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := newMockFullRepo()
	sess := &mockSessionRepo{}
	s := &ReminderService{repo: repo, sessions: sess, logger: logger, loc: DefaultLoc}
	return s, repo, sess
}

// ==========================================
// CompleteReminder
// ==========================================

func TestCompleteReminder_OneTime_DeletesReminder(t *testing.T) {
	s, repo, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminder(ctx, 1, "Buy milk", time.Now())
	if err := s.CompleteReminder(ctx, 1, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, id); !errors.Is(err, storage.ErrNotFound) {
		t.Error("one-time reminder should be deleted after completion")
	}
}

func TestCompleteReminder_DailyInterval_AdvancesToNextOccurrence(t *testing.T) {
	s, repo, _ := newTestService()
	ctx := context.Background()

	start := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	_ = s.AddRecurrentReminder(ctx, 1, "Daily standup", start, "24h")

	if err := s.CompleteReminder(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}

	updated, _ := repo.GetByID(ctx, 1)
	// Should be advanced to the future
	if !updated.NotifyAt.After(time.Now()) {
		t.Errorf("expected future time, got %v", updated.NotifyAt)
	}
	// LastMessageID should be reset so scheduler sends a new notification
	if updated.LastMessageID != 0 {
		t.Errorf("expected LastMessageID=0 after completion, got %d", updated.LastMessageID)
	}
}

func TestCompleteReminder_Weekdays_AdvancesToCorrectDay(t *testing.T) {
	s, repo, _ := newTestService()
	ctx := context.Background()

	// Set up a reminder for Mon+Wed (bitmask: 1<<0 | 1<<2 = 5)
	now := time.Now().In(DefaultLoc)
	rem := &storage.Reminder{ChatID: 1, Text: "Weekday task", NotifyAt: now, Weekdays: 5}
	_ = repo.Add(ctx, rem)

	if err := s.CompleteReminder(ctx, 1, rem.ID); err != nil {
		t.Fatal(err)
	}

	updated, _ := repo.GetByID(ctx, rem.ID)
	nextWd := updated.NotifyAt.In(DefaultLoc).Weekday()

	// Next occurrence must be Monday or Wednesday
	if nextWd != time.Monday && nextWd != time.Wednesday {
		t.Errorf("expected next day to be Mon or Wed, got %v", nextWd)
	}
	if !updated.NotifyAt.After(time.Now()) {
		t.Errorf("expected future time, got %v", updated.NotifyAt)
	}
}

// TestCompleteReminder_Weekdays_PreservesTimeOfDay verifies that when a weekday
// reminder is completed, the next occurrence keeps the original hour:minute.
// Regression: before the fix, the time-of-day was replaced by time.Now().
func TestCompleteReminder_Weekdays_PreservesTimeOfDay(t *testing.T) {
	s, repo, _ := newTestService()
	ctx := context.Background()

	// Create a reminder for every weekday (mask=31: Mon-Fri) at exactly 09:00 MSK,
	// but set NotifyAt to yesterday at 09:00 to simulate a past notification.
	yesterday := time.Now().In(DefaultLoc).AddDate(0, 0, -1)
	nineAM := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(),
		9, 0, 0, 0, DefaultLoc)

	rem := &storage.Reminder{ChatID: 1, Text: "Morning standup", NotifyAt: nineAM, Weekdays: 31}
	_ = repo.Add(ctx, rem)

	if err := s.CompleteReminder(ctx, 1, rem.ID); err != nil {
		t.Fatal(err)
	}

	updated, _ := repo.GetByID(ctx, rem.ID)
	nextTime := updated.NotifyAt.In(DefaultLoc)

	// The hour and minute MUST be preserved from the original
	if nextTime.Hour() != 9 || nextTime.Minute() != 0 {
		t.Errorf("expected time 09:00, got %02d:%02d", nextTime.Hour(), nextTime.Minute())
	}
	if !updated.NotifyAt.After(time.Now()) {
		t.Errorf("expected future time, got %v", updated.NotifyAt)
	}
}

func TestCompleteReminder_WrongChatID_ReturnsError(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminder(ctx, 1, "Secret", time.Now())
	err := s.CompleteReminder(ctx, 2, id) // wrong chat
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCompleteReminder_NonExistent_ReturnsError(t *testing.T) {
	s, _, _ := newTestService()
	err := s.CompleteReminder(context.Background(), 1, 999)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ==========================================
// UpdateReminderText
// ==========================================

func TestUpdateReminderText_Success(t *testing.T) {
	s, repo, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminder(ctx, 1, "Old text", time.Now())
	if err := s.UpdateReminderText(ctx, 1, id, "New text"); err != nil {
		t.Fatal(err)
	}

	updated, _ := repo.GetByID(ctx, id)
	if updated.Text != "New text" {
		t.Errorf("expected 'New text', got %q", updated.Text)
	}
}

func TestUpdateReminderText_WrongChatID(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminder(ctx, 1, "Test", time.Now())
	err := s.UpdateReminderText(ctx, 2, id, "Hacked")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ==========================================
// RescheduleReminder
// ==========================================

func TestRescheduleReminder_Success(t *testing.T) {
	s, repo, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminder(ctx, 1, "Meeting", time.Now())
	newTime := time.Now().Add(24 * time.Hour)
	if err := s.RescheduleReminder(ctx, 1, id, newTime); err != nil {
		t.Fatal(err)
	}

	updated, _ := repo.GetByID(ctx, id)
	if !updated.NotifyAt.Equal(newTime) {
		t.Errorf("expected %v, got %v", newTime, updated.NotifyAt)
	}
}

func TestRescheduleReminder_WrongChatID(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminder(ctx, 1, "Test", time.Now())
	err := s.RescheduleReminder(ctx, 2, id, time.Now())
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ==========================================
// UpdateReminderInterval / Weekdays
// ==========================================

func TestUpdateReminderInterval_ClearsWeekdays(t *testing.T) {
	s, repo, _ := newTestService()
	ctx := context.Background()

	rem := &storage.Reminder{ChatID: 1, Text: "Test", NotifyAt: time.Now(), Weekdays: 5}
	_ = repo.Add(ctx, rem)

	if err := s.UpdateReminderInterval(ctx, 1, rem.ID, "24h"); err != nil {
		t.Fatal(err)
	}

	updated, _ := repo.GetByID(ctx, rem.ID)
	if updated.Weekdays != 0 {
		t.Errorf("expected weekdays=0, got %d", updated.Weekdays)
	}
	if updated.Interval != "24h" {
		t.Errorf("expected interval=24h, got %s", updated.Interval)
	}
}

func TestUpdateReminderWeekdays_ClearsInterval(t *testing.T) {
	s, repo, _ := newTestService()
	ctx := context.Background()

	rem := &storage.Reminder{ChatID: 1, Text: "Test", NotifyAt: time.Now(), Interval: "24h"}
	_ = repo.Add(ctx, rem)

	if err := s.UpdateReminderWeekdays(ctx, 1, rem.ID, 3); err != nil {
		t.Fatal(err)
	}

	updated, _ := repo.GetByID(ctx, rem.ID)
	if updated.Interval != "" {
		t.Errorf("expected empty interval, got %s", updated.Interval)
	}
	if updated.Weekdays != 3 {
		t.Errorf("expected weekdays=3, got %d", updated.Weekdays)
	}
}

func TestUpdateRecurrence_WrongChatID(t *testing.T) {
	s, _, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminder(ctx, 1, "Test", time.Now())
	if err := s.UpdateReminderInterval(ctx, 2, id, "1h"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound for interval, got %v", err)
	}
	if err := s.UpdateReminderWeekdays(ctx, 2, id, 1); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound for weekdays, got %v", err)
	}
}

// ==========================================

// ==========================================
// ParseIDFromState
// ==========================================

// ==========================================
// State management (ClearState clears pending text)
// ==========================================

// ==========================================
// Creation flow integration
// ==========================================

func TestCreationFlow_WithRecurrence(t *testing.T) {
	s, _, sess := newTestService()
	stateMgr := NewStateManager(sess, s.logger)
	ctx := context.Background()
	chatID := int64(1)

	// Step 1: Add reminder
	id, err := s.AddReminder(ctx, chatID, "Buy groceries", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	// Step 2: Store pending and set state
	_ = stateMgr.SetPendingReminder(ctx, chatID, id)
	_ = stateMgr.SetWaitingRecurrenceState(ctx, chatID)

	if sess.state != StateWaitingRecurrence {
		t.Errorf("expected state %q, got %q", StateWaitingRecurrence, sess.state)
	}

	// Step 3: Apply recurrence

	// 5. Get pending reminder to set interval
	pendID, _ := stateMgr.GetPendingReminder(ctx, chatID)
	if err := s.UpdateReminderInterval(ctx, chatID, pendID, "24h"); err != nil {
		t.Fatal(err)
	}

	// Step 4: Verify
	rems, _ := s.GetReminders(ctx, chatID)
	if len(rems) != 1 {
		t.Fatalf("expected 1 reminder, got %d", len(rems))
	}
	if rems[0].Interval != "24h" {
		t.Errorf("expected interval 24h, got %s", rems[0].Interval)
	}
}

func TestSnoozeReminder_Success(t *testing.T) {
	ctx := context.Background()
	s, store, _ := newTestService()
	chatID := int64(404)

	now := time.Now().UTC()
	id, _ := s.AddReminder(ctx, chatID, "Test Snooze", now)

	// Mark as notified manually (simulate scheduler)
	for i, r := range store.reminders {
		if r.ID == id {
			r.LastMessageID = 42
			store.reminders[i] = r
			break
		}
	}

	err := s.SnoozeReminder(ctx, chatID, id, 15*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rem, _ := s.GetReminder(ctx, id)
	if rem.LastMessageID != 0 {
		t.Errorf("expected LastMessageID to be 0, got %d", rem.LastMessageID)
	}

	expectedTime := now.Add(15 * time.Minute)
	diff := rem.NotifyAt.Sub(expectedTime)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("expected time %v, got %v", expectedTime, rem.NotifyAt)
	}
}

func TestSnoozeReminder_WrongChatID(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newTestService()

	id, _ := s.AddReminder(ctx, 1, "Test", time.Now())

	err := s.SnoozeReminder(ctx, 2, id, 15*time.Minute)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

type dummyTestStorage struct {
	repo storage.ReminderRepository
	sess storage.SessionRepository
}

func (d *dummyTestStorage) Reminders() storage.ReminderRepository { return d.repo }
func (d *dummyTestStorage) Sessions() storage.SessionRepository   { return d.sess }
func (d *dummyTestStorage) Friends() storage.FriendRepository     { return nil }
func (d *dummyTestStorage) Users() storage.UserRepository         { return nil }
func (d *dummyTestStorage) Close() error                          { return nil }

func TestNewReminderService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	store := &dummyTestStorage{
		repo: newMockFullRepo(),
		sess: &mockSessionRepo{},
	}
	s := NewReminderService(store, logger, nil)
	if s.DefaultLocation().String() != DefaultLoc.String() {
		t.Errorf("expected default loc %v", DefaultLoc)
	}

	customLoc := time.UTC
	s2 := NewReminderService(store, logger, customLoc)
	if s2.DefaultLocation().String() != "UTC" {
		t.Errorf("expected custom loc UTC, got %v", s2.DefaultLocation())
	}
}

func TestDeleteReminder_Success(t *testing.T) {
	s, repo, _ := newTestService()
	ctx := context.Background()

	id, _ := s.AddReminder(ctx, 1, "test", time.Now())
	if err := s.DeleteReminder(ctx, 1, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, id); err == nil {
		t.Errorf("expected error after delete, got nil")
	}
}

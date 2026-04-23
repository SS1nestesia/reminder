package core

import (
	"context"
	"log/slog"
	"testing"
)

func TestParseIDFromState(t *testing.T) {
	m := &StateManager{logger: slog.Default()}

	tests := []struct {
		state  string
		prefix string
		wantID int64
		wantOK bool
	}{
		{"editing:42", StateEditingPrefix, 42, true},
		{"editing:0", StateEditingPrefix, 0, true},
		{"reschedule:123", StateReschedulePrefix, 123, true},
		{"edit_repeat:7", StateEditRepeatPrefix, 7, true},
		{"weekdays:99", StateWeekdaysPrefix, 99, true},
		{"custom:5", StateCustomIntervalPrefix, 5, true},
		// Negative cases
		{"waiting_text", StateEditingPrefix, 0, false},  // wrong prefix
		{"editing:", StateEditingPrefix, 0, false},      // empty ID
		{"editing:abc", StateEditingPrefix, 0, false},   // non-numeric
		{"reschedule:42", StateEditingPrefix, 0, false}, // mismatched prefix
	}

	for _, tt := range tests {
		t.Run(tt.state+"_"+tt.prefix, func(t *testing.T) {
			id, ok := m.ParseIDFromState(tt.state, tt.prefix)
			if ok != tt.wantOK {
				t.Errorf("ParseIDFromState(%q, %q) ok = %v, want %v", tt.state, tt.prefix, ok, tt.wantOK)
			}
			if ok && id != tt.wantID {
				t.Errorf("ParseIDFromState(%q, %q) id = %d, want %d", tt.state, tt.prefix, id, tt.wantID)
			}
		})
	}
}

func TestClearState_ClearsPendingTextAndState(t *testing.T) {
	sess := &mockSessionRepo{}
	// Note: We inject a nil logger here for tests or a discard logger.
	// We didn't do this before in service_test, but we need it now.
	// Actually let's assume we can pass nil or use slog.Default() if needed.
	m := &StateManager{sessions: sess, logger: slog.Default()}
	ctx := context.Background()

	_ = m.SetWaitingForTextState(ctx, 1)
	_ = m.SetPendingText(ctx, 1, "test")

	if err := m.ClearState(ctx, 1); err != nil {
		t.Fatal(err)
	}

	if sess.state != "" {
		t.Errorf("expected state to be empty, got %q", sess.state)
	}
	if sess.pendingText != "" {
		t.Errorf("expected pending text to be empty, got %q", sess.pendingText)
	}
}

func TestCleanupSessions(t *testing.T) {
	sess := &mockSessionRepo{}
	m := &StateManager{sessions: sess, logger: slog.Default()}
	if err := m.CleanupSessions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !sess.cleaned {
		t.Error("expected sessions cleanup to be called")
	}
}

func TestSetSessionMessage_And_GetSessionMessage(t *testing.T) {
	sess := &mockSessionRepo{}
	m := &StateManager{sessions: sess, logger: slog.Default()}
	ctx := context.Background()

	err := m.SetSessionMessage(ctx, 100, 42)
	if err != nil {
		t.Fatalf("SetSessionMessage: %v", err)
	}

	got, err := m.GetSessionMessage(ctx, 100)
	if err != nil {
		t.Fatalf("GetSessionMessage: %v", err)
	}
	if got != 42 {
		t.Errorf("GetSessionMessage = %d, want 42", got)
	}
}

func TestSetTimezone_And_GetTimezone(t *testing.T) {
	sess := &mockSessionRepo{}
	m := &StateManager{sessions: sess, logger: slog.Default()}
	ctx := context.Background()

	_ = m.SetTimezone(ctx, 1, "Europe/Moscow")
	tz, err := m.GetTimezone(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if tz != "Europe/Moscow" {
		t.Errorf("got %q, want Europe/Moscow", tz)
	}
}

func TestSetWaitingStates(t *testing.T) {
	sess := &mockSessionRepo{}
	m := &StateManager{sessions: sess, logger: slog.Default()}
	ctx := context.Background()

	tests := []struct {
		name     string
		setFunc  func() error
		expected string
	}{
		{"text", func() error { return m.SetWaitingForTextState(ctx, 1) }, StateWaitingReminderText},
		{"time", func() error { return m.SetWaitingForTimeState(ctx, 1) }, StateWaitingReminderTime},
		{"recurrence", func() error { return m.SetWaitingRecurrenceState(ctx, 1) }, StateWaitingRecurrence},
		{"timezone", func() error { return m.SetWaitingTimezoneState(ctx, 1) }, StateWaitingTimezone},
		{"editing", func() error { return m.SetEditingState(ctx, 1, 42) }, StateEditingPrefix + "42"},
		{"reschedule", func() error { return m.SetRescheduleState(ctx, 1, 99) }, StateReschedulePrefix + "99"},
		{"edit_repeat", func() error { return m.SetEditRepeatState(ctx, 1, 7) }, StateEditRepeatPrefix + "7"},
		{"weekdays", func() error { return m.SetWaitingWeekdaysState(ctx, 1, 88) }, StateWeekdaysPrefix + "88"},
		{"custom_interval", func() error { return m.SetState(ctx, 1, StateCustomIntervalPrefix+"15") }, StateCustomIntervalPrefix + "15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setFunc(); err != nil {
				t.Fatal(err)
			}
			got, err := m.GetUserState(ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.expected {
				t.Errorf("after set: got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestResolveReminderID(t *testing.T) {
	sess := &mockSessionRepo{}
	m := &StateManager{sessions: sess, logger: slog.Default()}
	ctx := context.Background()

	// Test 1: ID from state prefix
	got := m.ResolveReminderID(ctx, 1, "edit_repeat:42", StateEditRepeatPrefix)
	if got != 42 {
		t.Errorf("from state prefix: got %d, want 42", got)
	}

	// Test 2: fallback to pending reminder
	sess.pendingID = 99
	got = m.ResolveReminderID(ctx, 1, "waiting_text")
	if got != 99 {
		t.Errorf("from pending: got %d, want 99", got)
	}

	// Test 3: nothing found
	sess.pendingID = 0
	got = m.ResolveReminderID(ctx, 1, "waiting_text", StateEditingPrefix)
	if got != 0 {
		t.Errorf("not found: got %d, want 0", got)
	}
}

func TestPendingText(t *testing.T) {
	sess := &mockSessionRepo{}
	m := &StateManager{sessions: sess, logger: slog.Default()}
	ctx := context.Background()

	_ = m.SetPendingText(ctx, 1, "hello world")
	text, err := m.GetPendingText(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello world" {
		t.Errorf("got %q, want %q", text, "hello world")
	}
}

func TestPendingReminder(t *testing.T) {
	sess := &mockSessionRepo{}
	m := &StateManager{sessions: sess, logger: slog.Default()}
	ctx := context.Background()

	_ = m.SetPendingReminder(ctx, 1, 55)
	id, err := m.GetPendingReminder(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if id != 55 {
		t.Errorf("got %d, want 55", id)
	}
}

func TestNewStateManager(t *testing.T) {
	sess := &mockSessionRepo{}
	m := NewStateManager(sess, nil)
	if m == nil {
		t.Fatal("NewStateManager returned nil")
	}
}


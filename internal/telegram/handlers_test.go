package telegram

import (
	"context"
	"testing"
	"time"

	"reminder-bot/internal/storage"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

// TestCallbackID is in keyboards_test.go

func TestParseWeekdayMask(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		dayBit   int
		expected int
	}{
		{"toggle monday on", 0, 1 << 1, 1 << 1},
		{"toggle monday off", 1 << 1, 1 << 1, 0},
		{"add tuesday to monday", 1 << 1, 1 << 2, (1 << 1) | (1 << 2)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.current ^ tt.dayBit
			if result != tt.expected {
				t.Errorf("got %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestParseWeekdayMaskActual(t *testing.T) {
	h := &Handlers{}
	// Empty message
	msg := &telego.Message{ReplyMarkup: &telego.InlineKeyboardMarkup{}}
	if got := h.parseWeekdayMask(msg); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}

	// Message with specific buttons
	msg.ReplyMarkup.InlineKeyboard = [][]telego.InlineKeyboardButton{
		{
			{Text: "✅ Пн", CallbackData: "wd:1"},
			{Text: "Вт", CallbackData: "wd:2"},
		},
	}
	if got := h.parseWeekdayMask(msg); got != 2 {
		t.Errorf("expected 2 (bit 1), got %d", got)
	}
}

func TestRegisterAll(t *testing.T) {
	h := &Handlers{
		creator: &CreatorHandlers{},
		editor:  &EditorHandlers{},
		list:    &ListHandlers{},
	}
	bot, _ := telego.NewBot("123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11")
	updates := make(<-chan telego.Update)
	bh, _ := th.NewBotHandler(bot, updates)
	h.RegisterAll(bh)
	// Registration should not panic
}

func TestBuildListText(t *testing.T) {
	loc := time.UTC

	tests := []struct {
		name      string
		reminders []storage.Reminder
		wantEmpty bool
	}{
		{
			name:      "empty list",
			reminders: nil,
			wantEmpty: true,
		},
		{
			name: "single reminder",
			reminders: []storage.Reminder{
				{ID: 1, Text: "Test", NotifyAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)},
			},
			wantEmpty: false,
		},
		{
			name: "multiple reminders",
			reminders: []storage.Reminder{
				{ID: 1, Text: "First", NotifyAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)},
				{ID: 2, Text: "Second", NotifyAt: time.Date(2026, 1, 2, 15, 30, 0, 0, time.UTC)},
			},
			wantEmpty: false,
		},
		{
			name: "reminder with HTML special chars",
			reminders: []storage.Reminder{
				{ID: 1, Text: "<b>bold</b> & \"quoted\"", NotifyAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)},
			},
			wantEmpty: false,
		},
		{
			name: "reminder with recurrence",
			reminders: []storage.Reminder{
				{ID: 1, Text: "Weekly", NotifyAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), Interval: "daily"},
			},
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newMockService()
			svc.userLoc = loc
			h := &Handlers{service: svc}
			result := h.buildListText(context.Background(), 123, tt.reminders)
			if tt.wantEmpty {
				if result != MsgEmptyList {
					t.Errorf("expected empty message, got: %s", result)
				}
			} else {
				if result == "" || result == MsgEmptyList {
					t.Errorf("expected non-empty list, got: %s", result)
				}
			}
		})
	}
}

func TestCallbackConstants(t *testing.T) {
	// Verify all constants have expected values (protects against typos)
	expectations := map[string]string{
		"CBAddReminder":         "add_reminder",
		"CBListReminders":       "list_reminders",
		"CBBackToMenu":          "back_to_menu",
		"CBCancel":              "cancel",
		"CBSetupTimezone":       "setup_timezone",
		"CBPrefixConfirmDelete": "confirm_delete:",
		"CBPrefixDelete":        "delete:",
		"CBPrefixEditText":      "edit_text:",
		"CBPrefixEditTime":      "edit_time:",
		"CBPrefixEditRepeat":    "edit_repeat:",
		"CBPrefixView":          "view:",
		"CBPrefixDone":          "done:",
		"CBPrefixReschedule":    "reschedule:",
		"CBPrefixSnoozeMenu":    "snooze_menu:",
		"CBPrefixSnooze":        "snooze:",
		"CBPrefixSnoozeBack":    "snooze_back:",
		"CBPrefixQuick":         "quick:",
		"CBPrefixRepeat":        "repeat:",
		"CBPrefixWeekday":       "wd:",
		"CBPrefixTimezone":      "tz:",
	}

	actuals := map[string]string{
		"CBAddReminder":         CBAddReminder,
		"CBListReminders":       CBListReminders,
		"CBBackToMenu":          CBBackToMenu,
		"CBCancel":              CBCancel,
		"CBSetupTimezone":       CBSetupTimezone,
		"CBPrefixConfirmDelete": CBPrefixConfirmDelete,
		"CBPrefixDelete":        CBPrefixDelete,
		"CBPrefixEditText":      CBPrefixEditText,
		"CBPrefixEditTime":      CBPrefixEditTime,
		"CBPrefixEditRepeat":    CBPrefixEditRepeat,
		"CBPrefixView":          CBPrefixView,
		"CBPrefixDone":          CBPrefixDone,
		"CBPrefixReschedule":    CBPrefixReschedule,
		"CBPrefixSnoozeMenu":    CBPrefixSnoozeMenu,
		"CBPrefixSnooze":        CBPrefixSnooze,
		"CBPrefixSnoozeBack":    CBPrefixSnoozeBack,
		"CBPrefixQuick":         CBPrefixQuick,
		"CBPrefixRepeat":        CBPrefixRepeat,
		"CBPrefixWeekday":       CBPrefixWeekday,
		"CBPrefixTimezone":      CBPrefixTimezone,
	}

	for name, expected := range expectations {
		actual := actuals[name]
		if actual != expected {
			t.Errorf("%s = %q, want %q", name, actual, expected)
		}
	}
}

func TestNewHandlers(t *testing.T) {
	svc := newMockService()
	state := newMockState()
	parser := newMockParser()

	h := NewHandlers(nil, svc, &mockFriendService{}, parser, state, "", nil)

	if h == nil {
		t.Fatal("NewHandlers returned nil")
	}
	if h.creator == nil {
		t.Error("creator sub-handler is nil")
	}
	if h.editor == nil {
		t.Error("editor sub-handler is nil")
	}
	if h.list == nil {
		t.Error("list sub-handler is nil")
	}
}

func TestMockServiceInterface(t *testing.T) {
	// Compile-time check that mockService satisfies ReminderServicer
	var _ ReminderServicer = (*mockService)(nil)
}

func TestMockStateInterface(t *testing.T) {
	// Compile-time check that mockState satisfies StateManagerr
	var _ StateManagerr = (*mockState)(nil)
}

func TestMockParserInterface(t *testing.T) {
	// Compile-time check that mockParser satisfies Parserr
	var _ Parserr = (*mockParser)(nil)
}

package telegram

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"reminder-bot/internal/storage"

	"github.com/mymmrac/telego"
)

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// ==========================================
// WeekdaysKeyboard
// ==========================================

func TestWeekdaysKeyboard_NoSelection(t *testing.T) {
	kb := WeekdaysKeyboard(0).(*telego.InlineKeyboardMarkup)

	// 3 rows: 4 days, 3 days, "Готово" button
	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(kb.InlineKeyboard))
	}

	// No day should have ✅ prefix
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.HasPrefix(btn.CallbackData, "wd:") && btn.CallbackData != "wd:done" {
				if strings.HasPrefix(btn.Text, "✅") {
					t.Errorf("no days should be selected, but %q is marked", btn.Text)
				}
			}
		}
	}
}

func TestWeekdaysKeyboard_SelectionMatchesMask(t *testing.T) {
	// Mon + Wed + Fri = 1<<0 | 1<<2 | 1<<4 = 21
	mask := 1 | 4 | 16
	kb := WeekdaysKeyboard(mask).(*telego.InlineKeyboardMarkup)

	selected := map[int]bool{}
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if !strings.HasPrefix(btn.CallbackData, "wd:") || btn.CallbackData == "wd:done" {
				continue
			}
			id, _ := strconv.Atoi(strings.TrimPrefix(btn.CallbackData, "wd:"))
			if strings.HasPrefix(btn.Text, "✅") {
				selected[id] = true
			}
		}
	}

	// Should have exactly Mon(0), Wed(2), Fri(4) selected
	if len(selected) != 3 || !selected[0] || !selected[2] || !selected[4] {
		t.Errorf("expected Mon,Wed,Fri selected, got %v", selected)
	}
}

func TestWeekdaysKeyboard_DoneButtonExists(t *testing.T) {
	kb := WeekdaysKeyboard(0).(*telego.InlineKeyboardMarkup)

	found := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == "wd:done" {
				found = true
				if !strings.Contains(btn.Text, "Готово") {
					t.Errorf("done button text unexpected: %q", btn.Text)
				}
			}
		}
	}
	if !found {
		t.Error("'Готово' button not found")
	}
}

// ==========================================
// ListKeyboard
// ==========================================

func TestListKeyboard_Empty(t *testing.T) {
	kb := ListKeyboard(nil, nil).(*telego.InlineKeyboardMarkup)

	// Should have "Добавить" and "В меню"
	var callbacks []string
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			callbacks = append(callbacks, btn.CallbackData)
		}
	}

	hasAdd := false
	hasMenu := false
	for _, cb := range callbacks {
		if cb == "add_reminder" {
			hasAdd = true
		}
		if cb == "back_to_menu" {
			hasMenu = true
		}
	}
	if !hasAdd || !hasMenu {
		t.Errorf("empty list should have add_reminder and back_to_menu, got %v", callbacks)
	}
}

func TestListKeyboard_WithReminders_UsesViewCallbacks(t *testing.T) {
	rems := []storage.Reminder{
		{ID: 1, Text: "First", NotifyAt: mustParseTime("2026-04-10T10:00:00Z")},
		{ID: 2, Text: "Second", NotifyAt: mustParseTime("2026-04-11T15:00:00Z")},
	}
	kb := ListKeyboard(rems, nil).(*telego.InlineKeyboardMarkup)

	// Each reminder should have a view:<id> button
	viewCallbacks := 0
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.HasPrefix(btn.CallbackData, "view:") {
				viewCallbacks++
			}
		}
	}
	if viewCallbacks != 2 {
		t.Errorf("expected 2 view buttons, got %d", viewCallbacks)
	}
}

func TestListKeyboard_LongTextTruncated(t *testing.T) {
	longText := strings.Repeat("a", 100)
	rems := []storage.Reminder{
		{ID: 1, Text: longText, NotifyAt: mustParseTime("2026-04-10T10:00:00Z")},
	}
	kb := ListKeyboard(rems, nil).(*telego.InlineKeyboardMarkup)

	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.HasPrefix(btn.CallbackData, "view:") {
				runes := []rune(btn.Text)
				if len(runes) > 55 {
					t.Errorf("button text too long: %d runes", len(runes))
				}
				if !strings.HasSuffix(btn.Text, "...") {
					t.Error("truncated text should end with ...")
				}
			}
		}
	}
}

// ==========================================
// ReminderActionsKeyboard
// ==========================================

func TestReminderActionsKeyboard_ContainsAllActions(t *testing.T) {
	id := int64(42)
	kb := ReminderActionsKeyboard(id).(*telego.InlineKeyboardMarkup)

	idStr := strconv.FormatInt(id, 10)
	expected := map[string]bool{
		"edit_text:" + idStr:      false,
		"edit_time:" + idStr:      false,
		"edit_repeat:" + idStr:    false,
		"confirm_delete:" + idStr: false,
		"list_reminders":          false,
	}

	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if _, ok := expected[btn.CallbackData]; ok {
				expected[btn.CallbackData] = true
			}
		}
	}

	for cb, found := range expected {
		if !found {
			t.Errorf("missing action button: %s", cb)
		}
	}
}

// ==========================================
// CancelEditKeyboard / QuickTimeEditKeyboard / RecurrenceEditKeyboard
// ==========================================

func TestCancelEditKeyboard_CancelGoesToView(t *testing.T) {
	id := int64(7)
	kb := CancelEditKeyboard(id).(*telego.InlineKeyboardMarkup)

	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 1 {
		t.Fatal("expected 1 row with 1 button")
	}
	btn := kb.InlineKeyboard[0][0]
	if btn.CallbackData != "view:7" {
		t.Errorf("expected callback 'view:7', got %q", btn.CallbackData)
	}
}

func TestQuickTimeEditKeyboard_HasCancelToView(t *testing.T) {
	id := int64(10)
	kb := QuickTimeEditKeyboard(id).(*telego.InlineKeyboardMarkup)

	hasCancelToView := false
	hasQuickTime := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == "view:10" {
				hasCancelToView = true
			}
			if strings.HasPrefix(btn.CallbackData, "quick:") {
				hasQuickTime = true
			}
		}
	}
	if !hasCancelToView {
		t.Error("missing cancel button with view:10")
	}
	if !hasQuickTime {
		t.Error("missing quick time buttons")
	}
}

func TestRecurrenceEditKeyboard_HasCancelToView(t *testing.T) {
	id := int64(5)
	kb := RecurrenceEditKeyboard(id).(*telego.InlineKeyboardMarkup)

	hasCancelToView := false
	hasRepeatOptions := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == "view:5" {
				hasCancelToView = true
			}
			if strings.HasPrefix(btn.CallbackData, "repeat:") {
				hasRepeatOptions = true
			}
		}
	}
	if !hasCancelToView {
		t.Error("missing cancel button with view:5")
	}
	if !hasRepeatOptions {
		t.Error("missing repeat option buttons")
	}
}

// ==========================================
// ConfirmDeleteKeyboard
// ==========================================

func TestConfirmDeleteKeyboard_NoGoesToView(t *testing.T) {
	kb := ConfirmDeleteKeyboard(99).(*telego.InlineKeyboardMarkup)

	hasDelete := false
	hasViewCancel := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == "delete:99" {
				hasDelete = true
			}
			if btn.CallbackData == "view:99" {
				hasViewCancel = true
			}
		}
	}
	if !hasDelete {
		t.Error("missing delete:99 button")
	}
	if !hasViewCancel {
		t.Error("'Нет' should go to view:99")
	}
}

// ==========================================
// callbackID (handler utility)
// ==========================================

func TestCallbackID(t *testing.T) {
	tests := []struct {
		data   string
		prefix string
		wantID int64
		wantOK bool
	}{
		{"view:42", "view:", 42, true},
		{"delete:1", "delete:", 1, true},
		{"edit_text:100", "edit_text:", 100, true},
		{"view:abc", "view:", 0, false}, // non-numeric
		{"view:", "view:", 0, false},    // empty id
		{"other:42", "view:", 0, false}, // wrong prefix
		{"", "view:", 0, false},         // empty data
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.data, tt.prefix), func(t *testing.T) {
			id, ok := callbackID(tt.data, tt.prefix)
			if ok != tt.wantOK {
				t.Errorf("callbackID(%q, %q) ok = %v, want %v", tt.data, tt.prefix, ok, tt.wantOK)
			}
			if ok && id != tt.wantID {
				t.Errorf("callbackID(%q, %q) = %d, want %d", tt.data, tt.prefix, id, tt.wantID)
			}
		})
	}
}

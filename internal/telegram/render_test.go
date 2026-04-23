package telegram

import (
	"strings"
	"testing"

	"github.com/mymmrac/telego"
)

// Tests for keyboard functions NOT covered in keyboards_test.go

func TestMainMenuKeyboard_WithTimezone(t *testing.T) {
	kb := MainMenuKeyboard("Europe/Moscow", 0).(*telego.InlineKeyboardMarkup)

	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(kb.InlineKeyboard))
	}

	tzBtn := kb.InlineKeyboard[0][0]
	if !strings.Contains(tzBtn.Text, "Europe/Moscow") {
		t.Errorf("tz button text = %q, want contains Europe/Moscow", tzBtn.Text)
	}
}

func TestMainMenuKeyboard_WithoutTimezone(t *testing.T) {
	kb := MainMenuKeyboard("", 0).(*telego.InlineKeyboardMarkup)

	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(kb.InlineKeyboard))
	}

	tzBtn := kb.InlineKeyboard[0][0]
	if tzBtn.Text != "🌐 Настроить время (обязательно)" {
		t.Errorf("tz button text = %q, want setup prompt", tzBtn.Text)
	}
}

func TestMainMenuKeyboard_WithPendingCount(t *testing.T) {
	kb := MainMenuKeyboard("Europe/Moscow", 3).(*telego.InlineKeyboardMarkup)

	friendsBtn := kb.InlineKeyboard[2][0]
	if !strings.Contains(friendsBtn.Text, "Друзья (3)") {
		t.Errorf("friends button text = %q, want contains (3)", friendsBtn.Text)
	}
}

func TestCancelKeyboard_HasCancelButton(t *testing.T) {
	kb := CancelKeyboard().(*telego.InlineKeyboardMarkup)
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 1 {
		t.Fatal("expected 1 row with 1 button")
	}
	if kb.InlineKeyboard[0][0].CallbackData != CBCancel {
		t.Errorf("callback data = %q, want %q", kb.InlineKeyboard[0][0].CallbackData, CBCancel)
	}
}

func TestBackToMenuKeyboard_HasBackButton(t *testing.T) {
	kb := BackToMenuKeyboard().(*telego.InlineKeyboardMarkup)
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 1 {
		t.Fatal("expected 1 row with 1 button")
	}
	if kb.InlineKeyboard[0][0].CallbackData != CBBackToMenu {
		t.Errorf("callback data = %q, want %q", kb.InlineKeyboard[0][0].CallbackData, CBBackToMenu)
	}
}

func TestTimezoneQuickKeyboard_HasCitiesAndCancel(t *testing.T) {
	kb := TimezoneQuickKeyboard().(*telego.InlineKeyboardMarkup)

	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(kb.InlineKeyboard))
	}

	callbacks := collectCallbacks(kb)
	if !callbacks[CBCancel] {
		t.Error("expected cancel button")
	}
	if !callbacks["tz:Etc/GMT"] {
		t.Error("expected UTC timezone button")
	}
	if !callbacks["tz:Europe/Moscow"] {
		t.Error("expected Moscow timezone button")
	}
}

func TestQuickTimeKeyboard_HasTimeOptions(t *testing.T) {
	kb := QuickTimeKeyboard().(*telego.InlineKeyboardMarkup)

	callbacks := collectCallbacks(kb)
	expected := []string{"quick:10m", "quick:30m", "quick:1h", "quick:2h"}
	for _, cb := range expected {
		if !callbacks[cb] {
			t.Errorf("missing callback %q", cb)
		}
	}
}

func TestRecurrenceKeyboard_HasAllOptions(t *testing.T) {
	kb := RecurrenceKeyboard().(*telego.InlineKeyboardMarkup)

	callbacks := collectCallbacks(kb)
	expected := []string{"repeat:none", "repeat:24h", "repeat:168h", "repeat:weekdays", "repeat:custom"}
	for _, cb := range expected {
		if !callbacks[cb] {
			t.Errorf("missing callback %q", cb)
		}
	}
}

func TestNotificationKeyboard_HasDoneSnoozeReschedule(t *testing.T) {
	kb := NotificationKeyboard(42).(*telego.InlineKeyboardMarkup)

	callbacks := collectCallbacks(kb)
	if !callbacks["done:42"] {
		t.Error("missing done:42")
	}
	if !callbacks["snooze_menu:42"] {
		t.Error("missing snooze_menu:42")
	}
	if !callbacks["reschedule:42"] {
		t.Error("missing reschedule:42")
	}
}

func TestSnoozeOptionsKeyboard_HasAllDurations(t *testing.T) {
	kb := SnoozeOptionsKeyboard(7).(*telego.InlineKeyboardMarkup)

	callbacks := collectCallbacks(kb)
	expected := []string{"snooze:5m:7", "snooze:30m:7", "snooze:1h:7", "snooze:24h:7", "snooze_back:7"}
	for _, cb := range expected {
		if !callbacks[cb] {
			t.Errorf("missing callback %q", cb)
		}
	}
}

func TestRecurrenceEditKeyboard_AllCallbacks(t *testing.T) {
	kb := RecurrenceEditKeyboard(3).(*telego.InlineKeyboardMarkup)

	callbacks := collectCallbacks(kb)
	expected := []string{"repeat:none", "repeat:24h", "repeat:168h", "repeat:weekdays", "repeat:custom", "view:3"}
	for _, cb := range expected {
		if !callbacks[cb] {
			t.Errorf("missing callback %q", cb)
		}
	}
}

// collectCallbacks gathers all callback data values from an inline keyboard.
func collectCallbacks(kb *telego.InlineKeyboardMarkup) map[string]bool {
	result := make(map[string]bool)
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			result[btn.CallbackData] = true
		}
	}
	return result
}

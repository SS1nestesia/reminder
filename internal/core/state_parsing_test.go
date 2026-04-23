package core

import (
	"log/slog"
	"testing"

)

func TestStateParsingHelpers(t *testing.T) {
	logger := slog.Default()
	sm := NewStateManager(nil, logger)

	t.Run("ParseIDFromState", func(t *testing.T) {
		id, ok := sm.ParseIDFromState("some_prefix:42", "some_prefix:")
		if !ok || id != 42 {
			t.Errorf("expected ok=true, id=42, got %v %d", ok, id)
		}
		
		_, ok = sm.ParseIDFromState("some_prefix:abc", "some_prefix:")
		if ok {
			t.Errorf("expected ok=false for invalid id")
		}
	})

	t.Run("PrefixParsers", func(t *testing.T) {
		id, ok := sm.ParseEditingID("editing:42")
		if !ok || id != 42 { t.Errorf("failed ParseEditingID") }

		id, ok = sm.ParseRescheduleID("reschedule:42")
		if !ok || id != 42 { t.Errorf("failed ParseRescheduleID") }

		id, ok = sm.ParseEditRepeatID("edit_repeat:42")
		if !ok || id != 42 { t.Errorf("failed ParseEditRepeatID") }

		id, ok = sm.ParseWeekdaysID("weekdays:42")
		if !ok || id != 42 { t.Errorf("failed ParseWeekdaysID") }

		id, ok = sm.ParseCustomIntervalID("custom:42")
		if !ok || id != 42 { t.Errorf("failed ParseCustomIntervalID") }
	})
}

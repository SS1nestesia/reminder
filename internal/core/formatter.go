// Package core contains the platform-agnostic business logic of the
// reminder bot: reminder CRUD and scheduling, FSM session management,
// NLP time/interval parsing, timezone resolution, recurrence rules,
// friend management and notification orchestration.
//
// Everything in this package is pure Go with no dependency on the
// Telegram SDK. It can be driven by any front-end that speaks to the
// [storage.Storage] interface.
package core

import (
	"strings"
	"time"

	"reminder-bot/internal/storage"
)

// FormatRecurrence returns a human-readable description of a reminder's recurrence pattern.
func FormatRecurrence(r storage.Reminder) string {
	if r.Weekdays != 0 {
		var days []string
		names := []string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"}
		for i := 0; i < 7; i++ {
			if (r.Weekdays & (1 << uint(i))) != 0 {
				days = append(days, names[i])
			}
		}
		return "Дни: " + strings.Join(days, ", ")
	}
	if r.Interval != "" {
		d, _ := time.ParseDuration(r.Interval)
		if d == 24*time.Hour {
			return "Каждый день"
		}
		if d == 168*time.Hour {
			return "Каждую неделю"
		}
		return "Каждые " + r.Interval
	}
	return "Нет"
}

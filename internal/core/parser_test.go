package core

import (
	"testing"
	"time"

	"reminder-bot/internal/storage"
)

func TestFormatRecurrence(t *testing.T) {
	tests := []struct {
		name     string
		reminder storage.Reminder
		want     string
	}{
		{"none", storage.Reminder{}, "Нет"},
		{"daily", storage.Reminder{Interval: "24h"}, "Каждый день"},
		{"weekly", storage.Reminder{Interval: "168h"}, "Каждую неделю"},
		{"custom interval", storage.Reminder{Interval: "2h0m0s"}, "Каждые 2h0m0s"},
		{"single weekday (Mon)", storage.Reminder{Weekdays: 1}, "Дни: Пн"},
		{"multiple weekdays (Mon+Wed+Fri)", storage.Reminder{Weekdays: 1 | 4 | 16}, "Дни: Пн, Ср, Пт"},
		{"all weekdays", storage.Reminder{Weekdays: 127}, "Дни: Пн, Вт, Ср, Чт, Пт, Сб, Вс"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRecurrence(tt.reminder)
			if got != tt.want {
				t.Errorf("FormatRecurrence() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	p := NewParser(DefaultLoc)

	tests := []struct {
		input   string
		wantErr bool
	}{
		{"через 10 минут", false},
		{"через 2 часа", false},
		{"завтра в 15:00", false},
		{"25 марта в 14:30", false}, // past month → next year
		{"мусор", true},
		{"вчера", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := p.ParseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && result.Before(time.Now()) {
				t.Errorf("ParseTime(%q) returned past time: %v", tt.input, result)
			}
		})
	}
}

func TestParseInterval(t *testing.T) {
	p := NewParser(DefaultLoc)

	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		// Go duration format
		{"15m", "15m0s", false},
		{"2h", "2h0m0s", false},
		// Russian natural language
		{"2 часа", "2h0m0s", false},
		{"3 дня", "72h0m0s", false},
		{"15 минут", "15m0s", false},
		// Combined
		{"1ч 30мин", "1h30m0s", false},
		// Errors
		{"0s", "", true},
		{"мусор", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := p.ParseInterval(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseInterval(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseInterval(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

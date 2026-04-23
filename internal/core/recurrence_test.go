package core

import (
	"reminder-bot/internal/storage"
	"testing"
	"time"
)

func TestNextOccurrence(t *testing.T) {
	locMSK := time.FixedZone("MSK", 3*60*60)

	// 2024-05-13 is a Monday
	baseTime := time.Date(2024, 5, 13, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		reminder storage.Reminder
		now      time.Time
		wantTime time.Time
		wantOk   bool
	}{
		{
			name: "One-time reminder",
			reminder: storage.Reminder{
				NotifyAt: baseTime,
			},
			now:    baseTime,
			wantOk: false,
		},
		{
			name: "Daily interval",
			reminder: storage.Reminder{
				NotifyAt: baseTime,
				Interval: "24h",
			},
			now:      baseTime,
			wantTime: baseTime.Add(24 * time.Hour),
			wantOk:   true,
		},
		{
			name: "Weekly weekdays (Mon, Wed) from Monday morning",
			reminder: storage.Reminder{
				NotifyAt: baseTime, // Monday 10:00
				Weekdays: 1 | 4,    // Monday (1<<0) | Wednesday (1<<2)
			},
			now:      baseTime,
			wantTime: baseTime.AddDate(0, 0, 2), // Wednesday 10:00
			wantOk:   true,
		},
		{
			name: "Weekly weekdays (Mon, Wed) from Wednesday afternoon",
			reminder: storage.Reminder{
				NotifyAt: baseTime, // Monday 10:00
				Weekdays: 1 | 4,    // Monday (1<<0) | Wednesday (1<<2)
			},
			now:      baseTime.AddDate(0, 0, 2).Add(1 * time.Hour), // Wednesday 11:00
			wantTime: baseTime.AddDate(0, 0, 7),                    // Next Monday 10:00
			wantOk:   true,
		},
		{
			name: "Sunday to Monday transition",
			reminder: storage.Reminder{
				NotifyAt: time.Date(2024, 5, 12, 10, 0, 0, 0, time.UTC), // Sunday 10:00
				Weekdays: 1,                                             // Monday
			},
			now:      time.Date(2024, 5, 12, 11, 0, 0, 0, time.UTC),
			wantTime: time.Date(2024, 5, 13, 10, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name: "Weekly weekdays (today) from morning",
			reminder: storage.Reminder{
				NotifyAt: time.Date(2024, 5, 13, 10, 0, 0, 0, time.UTC), // Monday 10:00
				Weekdays: 1,                                             // Monday
			},
			now:      time.Date(2024, 5, 13, 07, 0, 0, 0, time.UTC), // Monday 07:00
			wantTime: time.Date(2024, 5, 13, 10, 0, 0, 0, time.UTC), // Monday 10:00 (Today!)
			wantOk:   true,
		},
		{
			name: "Daily interval (today) from morning",
			reminder: storage.Reminder{
				NotifyAt: time.Date(2024, 5, 13, 10, 0, 0, 0, time.UTC), // Monday 10:00
				Interval: "24h",
			},
			now:      time.Date(2024, 5, 13, 07, 0, 0, 0, time.UTC), // Monday 07:00
			wantTime: time.Date(2024, 5, 13, 10, 0, 0, 0, time.UTC), // Monday 10:00 (Today!)
			wantOk:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTime, gotOk := NextOccurrence(tt.reminder, locMSK, tt.now)
			if gotOk != tt.wantOk {
				t.Fatalf("got ok %v, want %v", gotOk, tt.wantOk)
			}
			if gotOk && !gotTime.Equal(tt.wantTime) {
				t.Errorf("got time %v, want %v", gotTime, tt.wantTime)
			}
		})
	}
}

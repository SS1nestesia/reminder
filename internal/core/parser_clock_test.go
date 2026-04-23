package core

import (
	"errors"
	"testing"
	"time"
)

// fixedClock is a Clock implementation that always returns the same time.
// It lets us exercise ParseTime deterministically without sleeping or
// racing with wall-clock drift.
type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func TestParser_ParseTime_Deterministic_Relative(t *testing.T) {
	// Arbitrary reference: 2026-03-15 10:00 MSK (UTC+3 → 07:00 UTC)
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, DefaultLoc)
	p := NewParserWithClock(DefaultLoc, fixedClock{now: now})

	got, err := p.ParseTime("через 30 минут")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := now.Add(30 * time.Minute).UTC()
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParser_ParseTime_EmptyInput_ReturnsSentinel(t *testing.T) {
	p := NewParser(DefaultLoc)
	for _, input := range []string{"", "   ", "\t\n"} {
		_, err := p.ParseTime(input)
		if !errors.Is(err, ErrEmptyInput) {
			t.Errorf("ParseTime(%q): expected ErrEmptyInput, got %v", input, err)
		}
	}
}

// TestParser_ParseTime_RecentPast_Rejected ensures that when a user types
// a time that is only a few hours in the past ("вчера", "10:00" earlier
// today), the parser returns ErrPastTime and does NOT roll it forward.
func TestParser_ParseTime_RecentPast_Rejected(t *testing.T) {
	// It's 15:00 MSK. "в 10:00" resolves to today 10:00 — 5h ago.
	now := time.Date(2026, 3, 15, 15, 0, 0, 0, DefaultLoc)
	p := NewParserWithClock(DefaultLoc, fixedClock{now: now})

	_, err := p.ParseTime("в 10:00")
	if !errors.Is(err, ErrPastTime) {
		t.Errorf("expected ErrPastTime for a past-today time, got %v", err)
	}
}

// TestParser_ParseTime_ExplicitPastDate_RollsForwardToNextYear verifies the
// documented behavior: an explicit calendar date more than 48h in the
// past is treated as "that date next year".
func TestParser_ParseTime_ExplicitPastDate_RollsForwardToNextYear(t *testing.T) {
	// It's April 10, 2026. "25 марта в 14:30" was ~16 days ago.
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, DefaultLoc)
	p := NewParserWithClock(DefaultLoc, fixedClock{now: now})

	got, err := p.ParseTime("25 марта в 14:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Year() != 2027 || got.Month() != time.March || got.Day() != 25 {
		t.Errorf("expected rolled-forward 2027-03-25, got %v", got)
	}
}

// TestParser_ParseInterval_TableDriven covers the documented Russian/
// English formats plus error paths in one place.
func TestParser_ParseInterval_TableDriven(t *testing.T) {
	p := NewParser(DefaultLoc)

	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"go-duration-hours", "24h", "24h0m0s", false},
		{"go-duration-minutes", "15m", "15m0s", false},
		{"ru-hours-full", "2 часа", "2h0m0s", false},
		{"ru-hours-short", "2ч", "2h0m0s", false},
		{"ru-days-full", "3 дня", "72h0m0s", false},
		{"ru-days-short", "3д", "72h0m0s", false},
		{"ru-minutes-short", "15 мин", "15m0s", false},
		{"ru-combined", "1ч 30мин", "1h30m0s", false},
		{"ru-two-days-one-hour", "2 дня 1 час", "49h0m0s", false},
		{"too-short", "30s", "", true}, // <1 minute
		{"empty", "", "", true},
		{"whitespace", "   ", "", true},
		{"garbage", "абвгд", "", true},
		{"unknown-unit", "5 попугаев", "", true},
		{"no-number", "час", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.ParseInterval(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParser_ParseInterval_InvalidIntegerOverflow(t *testing.T) {
	// 99999999999999999999 overflows int64. Must yield an error, not a panic.
	p := NewParser(DefaultLoc)
	if _, err := p.ParseInterval("99999999999999999999 мин"); err == nil {
		t.Error("expected error on integer overflow, got nil")
	}
}

package core

import (
	"context"
	"testing"
)

func TestParseTimezoneAlias(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"MSK", "Europe/Moscow"},
		{"мск", "Europe/Moscow"},
		{"EKT", "Asia/Yekaterinburg"},
		{"екб", "Asia/Yekaterinburg"},
		{"UTC", "UTC"},
		{"GMT", "UTC"},
		{"UTC+3", "Etc/GMT-3"},
		{"GMT+5", "Etc/GMT-5"},
		{"+7", "Etc/GMT-7"},
		{"-4", "Etc/GMT+4"},
		{"UTC-10", "Etc/GMT+10"},
		{"GMT  +  2", "Etc/GMT-2"},
		{"invalid", ""},
		{"Europe/Moscow", ""}, // Explicit IANA not parsed by alias
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseTimezoneAlias(tt.input)
			if result != tt.expected {
				t.Errorf("ParseTimezoneAlias(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetTimezoneName(t *testing.T) {
	// Moscow coordinates
	tz := GetTimezoneName(55.7558, 37.6173)
	if tz != "Europe/Moscow" {
		t.Errorf("expected Europe/Moscow, got %q", tz)
	}

	// Yekaterinburg coordinates
	tz2 := GetTimezoneName(56.8389, 60.6057)
	if tz2 != "Asia/Yekaterinburg" {
		t.Errorf("expected Asia/Yekaterinburg, got %q", tz2)
	}
}

func TestGeocodeCity_Error(t *testing.T) {
	// Simple test for not found (fake random city name that doesn't exist)
	_, _, err := GeocodeCity(context.Background(), "UnbelievableFakeCityNameThatDoesntExist123123")
	if err == nil {
		t.Error("expected error for fake city, but got none")
	}
}

func TestFormatTimezone(t *testing.T) {
	if got := FormatTimezone(""); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
	got := FormatTimezone("Europe/Moscow")
	if got == "Europe/Moscow" || got == "" || len(got) < 10 {
		t.Errorf("expected formatted timezone with alias, got %q", got)
	}
}

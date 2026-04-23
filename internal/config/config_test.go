package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear potential env vars
	os.Unsetenv("TOKEN")
	os.Unsetenv("DB_PATH")
	os.Unsetenv("TIMEZONE")
	os.Unsetenv("SCHEDULER_INTERVAL")

	cfg := Load()

	if cfg.DBPath != "reminder_bot.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "reminder_bot.db")
	}
	if cfg.Timezone != "Europe/Moscow" {
		t.Errorf("Timezone = %q, want %q", cfg.Timezone, "Europe/Moscow")
	}
	if cfg.SchedulerInterval != 10*time.Second {
		t.Errorf("SchedulerInterval = %v, want %v", cfg.SchedulerInterval, 10*time.Second)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	os.Setenv("TOKEN", "test-token-123")
	os.Setenv("DB_PATH", "/tmp/test.db")
	os.Setenv("TIMEZONE", "Asia/Tokyo")
	os.Setenv("SCHEDULER_INTERVAL", "30s")
	defer func() {
		os.Unsetenv("TOKEN")
		os.Unsetenv("DB_PATH")
		os.Unsetenv("TIMEZONE")
		os.Unsetenv("SCHEDULER_INTERVAL")
	}()

	cfg := Load()

	if cfg.Token != "test-token-123" {
		t.Errorf("Token = %q, want %q", cfg.Token, "test-token-123")
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/tmp/test.db")
	}
	if cfg.Timezone != "Asia/Tokyo" {
		t.Errorf("Timezone = %q, want %q", cfg.Timezone, "Asia/Tokyo")
	}
	if cfg.SchedulerInterval != 30*time.Second {
		t.Errorf("SchedulerInterval = %v, want %v", cfg.SchedulerInterval, 30*time.Second)
	}
}

func TestLoad_InvalidSchedulerInterval(t *testing.T) {
	os.Setenv("SCHEDULER_INTERVAL", "invalid")
	defer os.Unsetenv("SCHEDULER_INTERVAL")

	cfg := Load()

	// Should fall back to default
	if cfg.SchedulerInterval != 10*time.Second {
		t.Errorf("SchedulerInterval = %v, want %v (default on invalid)", cfg.SchedulerInterval, 10*time.Second)
	}
}

func TestLoad_TooSmallSchedulerInterval(t *testing.T) {
	os.Setenv("SCHEDULER_INTERVAL", "500ms")
	defer os.Unsetenv("SCHEDULER_INTERVAL")

	cfg := Load()

	// < 1s should fall back to default
	if cfg.SchedulerInterval != 10*time.Second {
		t.Errorf("SchedulerInterval = %v, want %v (minimum 1s)", cfg.SchedulerInterval, 10*time.Second)
	}
}

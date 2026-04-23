package config

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Token             string
	DBPath            string
	Timezone          string
	SchedulerInterval time.Duration
}

func Load() Config {
	_ = godotenv.Load()
	token := os.Getenv("TOKEN")
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "reminder_bot.db"
	}
	tz := os.Getenv("TIMEZONE")
	if tz == "" {
		tz = "Europe/Moscow"
	}

	schedulerInterval := 10 * time.Second
	if s := os.Getenv("SCHEDULER_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d >= 1*time.Second {
			schedulerInterval = d
		}
	}

	return Config{
		Token:             token,
		DBPath:            dbPath,
		Timezone:          tz,
		SchedulerInterval: schedulerInterval,
	}
}

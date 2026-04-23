// Package main is the entry point of the Reminder Bot Telegram application.
// It wires the config, storage, core business services, and telegram handlers
// into a runnable long-polling bot process.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"reminder-bot/internal/config"
	"reminder-bot/internal/core"
	"reminder-bot/internal/storage"
	"reminder-bot/internal/telegram"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()

	if cfg.Token == "" {
		logger.Error("TOKEN not found in environment")
		os.Exit(1)
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		logger.Warn("failed to load timezone, using default MSK", "timezone", cfg.Timezone, "error", err)
		loc = core.DefaultLoc
	}

	st, err := storage.NewSQLiteStorage(cfg.DBPath)
	if err != nil {
		logger.Error("failed to init storage", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := st.Close(); err != nil {
			logger.Error("failed to close storage", "error", err)
		}
	}()

	service := core.NewReminderService(st, logger, loc)
	friendService := core.NewFriendService(st, logger)

	botInstance, err := telego.NewBot(cfg.Token)
	if err != nil {
		logger.Error("failed to create bot", "error", err)
		os.Exit(1)
	}

	// Get bot username for invite links
	botUser, err := botInstance.GetMe(context.Background())
	botName := ""
	if err == nil && botUser != nil {
		botName = botUser.Username
	} else {
		logger.Warn("failed to get bot info, invite links may not work", "error", err)
	}

	stateManager := core.NewStateManager(st.Sessions(), logger)
	parser := core.NewParser(loc)
	handlers := telegram.NewHandlers(botInstance, service, friendService, parser, stateManager, botName, logger)
	notifier := telegram.NewTelegramNotifier(botInstance)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	updates, err := botInstance.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{
		Timeout: 60,
	})
	if err != nil {
		logger.Error("failed to start long polling", "error", err)
		os.Exit(1)
	}

	bh, err := th.NewBotHandler(botInstance, updates)
	if err != nil {
		logger.Error("failed to create bot handler", "error", err)
		os.Exit(1)
	}

	handlers.RegisterAll(bh)

	notificationManager := core.NewNotificationManager(st.Reminders(), logger)
	scheduler := core.NewScheduler(notificationManager, stateManager, notifier, cfg.SchedulerInterval, logger)
	go scheduler.Start(ctx)

	go func() {
		<-ctx.Done()
		logger.Info("shutting down bot...")
		if err := bh.Stop(); err != nil {
			logger.Error("failed to stop bot handler", "error", err)
		}
	}()

	logger.Info("bot is running", "timezone", cfg.Timezone)
	if err := bh.Start(); err != nil {
		logger.Error("bot handler terminated with error", "error", err)
		os.Exit(1)
	}
}

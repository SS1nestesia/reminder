package core

import (
	"context"
	"log/slog"
	"time"
)

type Scheduler struct {
	manager  *NotificationManager
	state    *StateManager
	notifier Notifier
	interval time.Duration
	logger   *slog.Logger
}

func NewScheduler(manager *NotificationManager, state *StateManager, notifier Notifier, interval time.Duration, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		manager:  manager,
		state:    state,
		notifier: notifier,
		interval: interval,
		logger:   logger,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.logger.Info("scheduler started", "interval", s.interval)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.run(ctx)
		case <-cleanupTicker.C:
			if err := s.state.CleanupSessions(ctx); err != nil {
				s.logger.Error("failed to cleanup sessions", "error", err)
			}
		}
	}
}

func (s *Scheduler) run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic in scheduler run", "recover", r)
		}
	}()

	s.manager.ProcessDueReminders(ctx, s.notifier)
}

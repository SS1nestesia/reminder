package core

import (
	"context"
	"log/slog"
	"time"

	"reminder-bot/internal/storage"
)

// Notifier defines the interface for sending notifications.
type Notifier interface {
	Notify(ctx context.Context, r storage.Reminder) (int, error)
	DeleteMessage(ctx context.Context, chatID int64, msgID int) error
}

// NotificationManager handles the cycle of processing due reminders and sending notifications.
type NotificationManager struct {
	repo   storage.ReminderRepository
	logger *slog.Logger
}

func NewNotificationManager(repo storage.ReminderRepository, logger *slog.Logger) *NotificationManager {
	return &NotificationManager{
		repo:   repo,
		logger: logger,
	}
}

func (m *NotificationManager) ProcessDueReminders(ctx context.Context, n Notifier) {
	reminders, err := m.repo.GetDue(ctx, time.Now())
	if err != nil {
		m.logger.Error("failed to get due reminders", "error", err)
		return
	}

	for _, r := range reminders {
		// Mark as notified immediately to avoid double processing if Notify takes time
		nextTry := time.Now().Add(1 * time.Minute)
		if err := m.repo.MarkAsNotified(ctx, r.ID, nextTry); err != nil {
			m.logger.Error("failed to mark reminder as notified", "id", r.ID, "error", err)
			continue
		}

		// Delete old notification message if exists
		if r.LastMessageID != 0 {
			if err := n.DeleteMessage(ctx, r.ChatID, r.LastMessageID); err != nil {
				m.logger.Warn("failed to delete old message", "chat_id", r.ChatID, "msg_id", r.LastMessageID, "error", err)
			}
		}

		// Send new notification
		newMsgID, err := n.Notify(ctx, r)
		if err != nil {
			m.logger.Error("failed to notify user", "chat_id", r.ChatID, "id", r.ID, "error", err)
			continue
		}

		// Update reminder with new message ID
		r.LastMessageID = newMsgID
		if err := m.repo.Update(ctx, &r); err != nil {
			m.logger.Error("failed to update reminder with last message id", "id", r.ID, "error", err)
		}
	}
}

package core

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"reminder-bot/internal/storage"
)

type dummyNotifier struct{}

func (d *dummyNotifier) Notify(ctx context.Context, item storage.Reminder) (int, error) { return 0, nil }
func (d *dummyNotifier) DeleteMessage(ctx context.Context, chatID int64, messageID int) error {
	return nil
}

type dummyReminderRepo struct{}

func (d *dummyReminderRepo) Add(ctx context.Context, r *storage.Reminder) error { return nil }
func (d *dummyReminderRepo) GetByID(ctx context.Context, id int64) (*storage.Reminder, error) {
	return nil, nil
}
func (d *dummyReminderRepo) GetByChatID(ctx context.Context, chatID int64) ([]storage.Reminder, error) {
	return nil, nil
}
func (d *dummyReminderRepo) Delete(ctx context.Context, chatID, id int64) error { return nil }
func (d *dummyReminderRepo) DeleteByID(ctx context.Context, id int64) error     { return nil }
func (d *dummyReminderRepo) Update(ctx context.Context, r *storage.Reminder) error { return nil }
func (d *dummyReminderRepo) GetDue(ctx context.Context, at time.Time) ([]storage.Reminder, error) {
	return nil, nil
}
func (d *dummyReminderRepo) MarkAsNotified(ctx context.Context, id int64, nextTime time.Time) error {
	return nil
}
func (d *dummyReminderRepo) GetByAuthorAndTarget(ctx context.Context, authorID, targetChatID int64) ([]storage.Reminder, error) {
	return nil, nil
}
func (d *dummyReminderRepo) GetFriendReminders(ctx context.Context, chatID int64) ([]storage.Reminder, error) {
	return nil, nil
}
func (d *dummyReminderRepo) ClearAuthor(ctx context.Context, authorID int64, targetChatID int64) error {
	return nil
}

type dummySessionRepo struct{}

func (d *dummySessionRepo) SetState(ctx context.Context, chatID int64, state string) error {
	return nil
}
func (d *dummySessionRepo) GetState(ctx context.Context, chatID int64) (string, error) {
	return "", nil
}
func (d *dummySessionRepo) DeleteState(ctx context.Context, chatID int64) error { return nil }
func (d *dummySessionRepo) SetPendingText(ctx context.Context, chatID int64, text string) error {
	return nil
}
func (d *dummySessionRepo) GetPendingText(ctx context.Context, chatID int64) (string, error) {
	return "", nil
}
func (d *dummySessionRepo) ClearPendingText(ctx context.Context, chatID int64) error { return nil }
func (d *dummySessionRepo) SetSessionMessageID(ctx context.Context, chatID int64, msgID int) error {
	return nil
}
func (d *dummySessionRepo) GetSessionMessageID(ctx context.Context, chatID int64) (int, error) {
	return 0, nil
}
func (d *dummySessionRepo) SetPendingReminderID(ctx context.Context, chatID int64, id int64) error {
	return nil
}
func (d *dummySessionRepo) GetPendingReminderID(ctx context.Context, chatID int64) (int64, error) {
	return 0, nil
}
func (d *dummySessionRepo) SetTimezone(ctx context.Context, chatID int64, tz string) error {
	return nil
}
func (d *dummySessionRepo) GetTimezone(ctx context.Context, chatID int64) (string, error) {
	return "", nil
}
func (d *dummySessionRepo) Cleanup(ctx context.Context, threshold time.Time) error { return nil }

func TestScheduler_Start(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stateManager := NewStateManager(&dummySessionRepo{}, logger)
	notifManager := NewNotificationManager(&dummyReminderRepo{}, logger)
	notifier := &dummyNotifier{}

	scheduler := NewScheduler(notifManager, stateManager, notifier, 10*time.Millisecond, logger)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel the context after a short delay so that the loop stops
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	scheduler.Start(ctx)
}

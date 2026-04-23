package core

import (
	"context"
	"log/slog"
	"time"

	"reminder-bot/internal/storage"
)

type ReminderService struct {
	repo     storage.ReminderRepository
	sessions storage.SessionRepository
	logger   *slog.Logger
	loc      *time.Location
}

func NewReminderService(s storage.Storage, logger *slog.Logger, loc *time.Location) *ReminderService {
	if loc == nil {
		loc = DefaultLoc
	}
	return &ReminderService{
		repo:     s.Reminders(),
		sessions: s.Sessions(),
		logger:   logger,
		loc:      loc,
	}
}

// DefaultLocation returns the service-wide default timezone.
func (s *ReminderService) DefaultLocation() *time.Location {
	return s.loc
}

func (s *ReminderService) GetUserLocation(ctx context.Context, chatID int64) *time.Location {
	tz, err := s.sessions.GetTimezone(ctx, chatID)
	if err == nil && tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return s.loc
}

// withReminder fetches a reminder by ID, verifies chatID ownership,
// calls the mutate function, and persists the result.
func (s *ReminderService) withReminder(ctx context.Context, chatID, id int64, mutate func(r *storage.Reminder) error) error {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if r.ChatID != chatID {
		return storage.ErrNotFound
	}
	if err := mutate(r); err != nil {
		return err
	}
	return s.repo.Update(ctx, r)
}

// --- Reminder CRUD ---

func (s *ReminderService) AddReminder(ctx context.Context, chatID int64, text string, notifyAt time.Time) (int64, error) {
	r := &storage.Reminder{
		ChatID:   chatID,
		Text:     text,
		NotifyAt: notifyAt,
	}
	if err := s.repo.Add(ctx, r); err != nil {
		return 0, err
	}
	return r.ID, nil
}

func (s *ReminderService) AddRecurrentReminder(ctx context.Context, chatID int64, text string, notifyAt time.Time, interval string) error {
	r := &storage.Reminder{
		ChatID:   chatID,
		Text:     text,
		NotifyAt: notifyAt,
		Interval: interval,
	}
	return s.repo.Add(ctx, r)
}

func (s *ReminderService) CompleteReminder(ctx context.Context, chatID int64, id int64) error {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if r.ChatID != chatID {
		return storage.ErrNotFound
	}

	userLoc := s.GetUserLocation(ctx, chatID)
	now := time.Now().In(userLoc)
	next, found := NextOccurrence(*r, userLoc, now)
	if !found {
		return s.repo.Delete(ctx, chatID, id)
	}

	r.NotifyAt = next.UTC()
	r.LastMessageID = 0
	return s.repo.Update(ctx, r)
}

func (s *ReminderService) GetReminders(ctx context.Context, chatID int64) ([]storage.Reminder, error) {
	return s.repo.GetByChatID(ctx, chatID)
}

func (s *ReminderService) GetReminder(ctx context.Context, id int64) (*storage.Reminder, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ReminderService) DeleteReminder(ctx context.Context, chatID int64, id int64) error {
	return s.repo.Delete(ctx, chatID, id)
}

func (s *ReminderService) UpdateReminderText(ctx context.Context, chatID int64, id int64, text string) error {
	return s.withReminder(ctx, chatID, id, func(r *storage.Reminder) error {
		r.Text = text
		return nil
	})
}

func (s *ReminderService) UpdateReminderInterval(ctx context.Context, chatID int64, id int64, interval string) error {
	return s.withReminder(ctx, chatID, id, func(r *storage.Reminder) error {
		r.Interval = interval
		r.Weekdays = 0
		return nil
	})
}

func (s *ReminderService) UpdateReminderWeekdays(ctx context.Context, chatID int64, id int64, weekdays int) error {
	return s.withReminder(ctx, chatID, id, func(r *storage.Reminder) error {
		r.Weekdays = weekdays
		r.Interval = ""
		return nil
	})
}

func (s *ReminderService) RescheduleReminder(ctx context.Context, chatID int64, id int64, newTime time.Time) error {
	return s.withReminder(ctx, chatID, id, func(r *storage.Reminder) error {
		r.NotifyAt = newTime
		return nil
	})
}

func (s *ReminderService) SnoozeReminder(ctx context.Context, chatID int64, id int64, duration time.Duration) error {
	return s.withReminder(ctx, chatID, id, func(r *storage.Reminder) error {
		userLoc := s.GetUserLocation(ctx, chatID)
		r.NotifyAt = time.Now().In(userLoc).Add(duration).UTC()
		r.LastMessageID = 0 // Reset so we don't try to touch the old notification
		return nil
	})
}

var (
	DefaultLoc = time.FixedZone("MSK", 3*60*60)
)

// AddReminderForFriend creates a reminder owned by targetChatID but authored by authorChatID.
func (s *ReminderService) AddReminderForFriend(ctx context.Context, authorChatID, targetChatID int64, text string, notifyAt time.Time) (int64, error) {
	r := &storage.Reminder{
		ChatID:   targetChatID,
		AuthorID: authorChatID,
		Text:     text,
		NotifyAt: notifyAt,
	}
	if err := s.repo.Add(ctx, r); err != nil {
		return 0, err
	}
	return r.ID, nil
}

// GetFriendReminders returns reminders created by friends for the given user.
func (s *ReminderService) GetFriendReminders(ctx context.Context, chatID int64) ([]storage.Reminder, error) {
	return s.repo.GetFriendReminders(ctx, chatID)
}

// DeleteFriendReminder deletes a reminder that was created by a friend.
// Either the owner (chatID == r.ChatID) or the author (chatID == r.AuthorID) can delete.
func (s *ReminderService) DeleteFriendReminder(ctx context.Context, chatID int64, id int64) (*storage.Reminder, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Allow deletion by owner or author
	if r.ChatID != chatID && r.AuthorID != chatID {
		return nil, storage.ErrNotFound
	}
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return nil, err
	}
	return r, nil
}

// UpdateFriendReminderText updates text for a friend reminder. Allowed for both owner and author.
func (s *ReminderService) UpdateFriendReminderText(ctx context.Context, chatID int64, id int64, text string) (*storage.Reminder, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.ChatID != chatID && r.AuthorID != chatID {
		return nil, storage.ErrNotFound
	}
	r.Text = text
	if err := s.repo.Update(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// UpdateFriendReminderTime updates time for a friend reminder. Allowed for both owner and author.
func (s *ReminderService) UpdateFriendReminderTime(ctx context.Context, chatID int64, id int64, newTime time.Time) (*storage.Reminder, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.ChatID != chatID && r.AuthorID != chatID {
		return nil, storage.ErrNotFound
	}
	r.NotifyAt = newTime
	if err := s.repo.Update(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}



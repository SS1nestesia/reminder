package core

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"reminder-bot/internal/storage"
)

const (
	StateWaitingReminderText  = "waiting_text"
	StateWaitingReminderTime  = "waiting_time"
	StateWaitingRecurrence    = "waiting_repeat"
	StateCustomIntervalPrefix = "custom:"
	StateEditingPrefix        = "editing:"
	StateReschedulePrefix     = "reschedule:"
	StateEditRepeatPrefix     = "edit_repeat:"
	StateWeekdaysPrefix       = "weekdays:"
	StateWaitingTimezone      = "waiting_timezone"
)

// StateManager orchestrates user session states and pending data.
type StateManager struct {
	sessions storage.SessionRepository
	logger   *slog.Logger
}

func NewStateManager(sessions storage.SessionRepository, logger *slog.Logger) *StateManager {
	return &StateManager{
		sessions: sessions,
		logger:   logger,
	}
}

// --- State Management ---

func (m *StateManager) SetWaitingForTextState(ctx context.Context, chatID int64) error {
	return m.sessions.SetState(ctx, chatID, StateWaitingReminderText)
}

func (m *StateManager) SetWaitingForTimeState(ctx context.Context, chatID int64) error {
	return m.sessions.SetState(ctx, chatID, StateWaitingReminderTime)
}

func (m *StateManager) SetWaitingTimezoneState(ctx context.Context, chatID int64) error {
	return m.sessions.SetState(ctx, chatID, StateWaitingTimezone)
}

func (m *StateManager) SetEditingState(ctx context.Context, chatID, id int64) error {
	return m.sessions.SetState(ctx, chatID, StateEditingPrefix+strconv.FormatInt(id, 10))
}

func (m *StateManager) SetRescheduleState(ctx context.Context, chatID, id int64) error {
	return m.sessions.SetState(ctx, chatID, StateReschedulePrefix+strconv.FormatInt(id, 10))
}

func (m *StateManager) SetEditRepeatState(ctx context.Context, chatID, id int64) error {
	state := StateEditRepeatPrefix + strconv.FormatInt(id, 10)
	m.logger.Debug("SetEditRepeatState", "chat_id", chatID, "id", id, "state", state)
	return m.sessions.SetState(ctx, chatID, state)
}

func (m *StateManager) SetWaitingRecurrenceState(ctx context.Context, chatID int64) error {
	return m.sessions.SetState(ctx, chatID, StateWaitingRecurrence)
}

func (m *StateManager) SetWaitingWeekdaysState(ctx context.Context, chatID, id int64) error {
	if id == 0 {
		// Fallback: try to get from pending reminder
		pid, _ := m.sessions.GetPendingReminderID(ctx, chatID)
		if pid != 0 {
			id = pid
		}
	}
	prefix := StateWeekdaysPrefix
	if id != 0 {
		prefix = fmt.Sprintf("%s%d", StateWeekdaysPrefix, id)
	}
	return m.sessions.SetState(ctx, chatID, prefix)
}

func (m *StateManager) GetUserState(ctx context.Context, chatID int64) (string, error) {
	return m.sessions.GetState(ctx, chatID)
}

func (m *StateManager) SetState(ctx context.Context, chatID int64, state string) error {
	return m.sessions.SetState(ctx, chatID, state)
}

func (m *StateManager) ClearState(ctx context.Context, chatID int64) error {
	if err := m.sessions.ClearPendingText(ctx, chatID); err != nil {
		m.logger.Error("failed to clear pending text", "chat_id", chatID, "error", err)
	}
	return m.sessions.DeleteState(ctx, chatID)
}

func (m *StateManager) GetTimezone(ctx context.Context, chatID int64) (string, error) {
	return m.sessions.GetTimezone(ctx, chatID)
}

func (m *StateManager) SetTimezone(ctx context.Context, chatID int64, tz string) error {
	return m.sessions.SetTimezone(ctx, chatID, tz)
}

// --- Pending Data ---

func (m *StateManager) SetPendingText(ctx context.Context, chatID int64, text string) error {
	return m.sessions.SetPendingText(ctx, chatID, text)
}

func (m *StateManager) GetPendingText(ctx context.Context, chatID int64) (string, error) {
	return m.sessions.GetPendingText(ctx, chatID)
}

func (m *StateManager) SetPendingReminder(ctx context.Context, chatID, id int64) error {
	return m.sessions.SetPendingReminderID(ctx, chatID, id)
}

func (m *StateManager) GetPendingReminder(ctx context.Context, chatID int64) (int64, error) {
	return m.sessions.GetPendingReminderID(ctx, chatID)
}

// --- Session Message ID ---

func (m *StateManager) SetSessionMessage(ctx context.Context, chatID int64, msgID int) error {
	return m.sessions.SetSessionMessageID(ctx, chatID, msgID)
}

func (m *StateManager) GetSessionMessage(ctx context.Context, chatID int64) (int, error) {
	return m.sessions.GetSessionMessageID(ctx, chatID)
}

func (m *StateManager) ParseIDFromState(state, prefix string) (int64, bool) {
	if !strings.HasPrefix(state, prefix) {
		return 0, false
	}
	idStr := strings.TrimPrefix(state, prefix)
	id, err := strconv.ParseInt(idStr, 10, 64)
	return id, err == nil
}

func (m *StateManager) ParseEditingID(state string) (int64, bool) {
	return m.ParseIDFromState(state, StateEditingPrefix)
}

func (m *StateManager) ParseRescheduleID(state string) (int64, bool) {
	return m.ParseIDFromState(state, StateReschedulePrefix)
}

func (m *StateManager) ParseEditRepeatID(state string) (int64, bool) {
	return m.ParseIDFromState(state, StateEditRepeatPrefix)
}

func (m *StateManager) ParseWeekdaysID(state string) (int64, bool) {
	return m.ParseIDFromState(state, StateWeekdaysPrefix)
}

func (m *StateManager) ParseCustomIntervalID(state string) (int64, bool) {
	return m.ParseIDFromState(state, StateCustomIntervalPrefix)
}

// ResolveReminderID finds the target reminder ID by checking state prefixes, then falling back to pending reminder.
func (m *StateManager) ResolveReminderID(ctx context.Context, chatID int64, state string, prefixes ...string) int64 {
	for _, prefix := range prefixes {
		if id, ok := m.ParseIDFromState(state, prefix); ok {
			return id
		}
	}
	if id, _ := m.GetPendingReminder(ctx, chatID); id != 0 {
		return id
	}
	return 0
}

func (m *StateManager) CleanupSessions(ctx context.Context) error {
	cutoff := time.Now().Add(-24 * time.Hour)
	return m.sessions.Cleanup(ctx, cutoff)
}

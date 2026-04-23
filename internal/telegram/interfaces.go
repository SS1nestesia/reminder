package telegram

import (
	"context"
	"time"

	"github.com/mymmrac/telego"
	"reminder-bot/internal/storage"
)

// ReminderServicer defines the business-logic methods used by telegram handlers.
// Implemented by *core.ReminderService; mockable for unit tests.
type ReminderServicer interface {
	AddReminder(ctx context.Context, chatID int64, text string, notifyAt time.Time) (int64, error)
	AddReminderForFriend(ctx context.Context, authorChatID, targetChatID int64, text string, notifyAt time.Time) (int64, error)
	GetReminder(ctx context.Context, id int64) (*storage.Reminder, error)
	GetReminders(ctx context.Context, chatID int64) ([]storage.Reminder, error)
	GetFriendReminders(ctx context.Context, chatID int64) ([]storage.Reminder, error)
	DeleteReminder(ctx context.Context, chatID int64, id int64) error
	DeleteFriendReminder(ctx context.Context, chatID int64, id int64) (*storage.Reminder, error)
	CompleteReminder(ctx context.Context, chatID int64, id int64) error
	RescheduleReminder(ctx context.Context, chatID int64, id int64, newTime time.Time) error
	SnoozeReminder(ctx context.Context, chatID int64, id int64, duration time.Duration) error
	UpdateReminderText(ctx context.Context, chatID int64, id int64, text string) error
	UpdateReminderInterval(ctx context.Context, chatID int64, id int64, interval string) error
	UpdateReminderWeekdays(ctx context.Context, chatID int64, id int64, weekdays int) error
	UpdateFriendReminderText(ctx context.Context, chatID int64, id int64, text string) (*storage.Reminder, error)
	UpdateFriendReminderTime(ctx context.Context, chatID int64, id int64, newTime time.Time) (*storage.Reminder, error)
	GetUserLocation(ctx context.Context, chatID int64) *time.Location
}

// FriendServicer defines friend management methods used by telegram handlers.
type FriendServicer interface {
	SendFriendRequest(ctx context.Context, userID, friendID int64) error
	AcceptFriendRequest(ctx context.Context, fromUserID, toUserID int64) error
	RejectFriendRequest(ctx context.Context, fromUserID, toUserID int64) error
	RemoveFriend(ctx context.Context, userID, friendID int64) error
	GetFriends(ctx context.Context, userID int64) ([]storage.Friend, error)
	GetPendingRequests(ctx context.Context, userID int64) ([]storage.Friend, error)
	IsFriend(ctx context.Context, userID, friendID int64) (bool, error)
	HasPendingRequest(ctx context.Context, userID, friendID int64) (bool, error)
	UpsertUser(ctx context.Context, user *storage.User) error
	GetUserInfo(ctx context.Context, chatID int64) (*storage.User, error)
}

// StateManagerr defines session/state management methods used by telegram handlers.
// Signatures match *core.StateManager exactly.
type StateManagerr interface {
	// State
	GetUserState(ctx context.Context, chatID int64) (string, error)
	SetState(ctx context.Context, chatID int64, state string) error
	ClearState(ctx context.Context, chatID int64) error
	SetWaitingForTextState(ctx context.Context, chatID int64) error
	SetWaitingForTimeState(ctx context.Context, chatID int64) error
	SetWaitingRecurrenceState(ctx context.Context, chatID int64) error
	SetWaitingWeekdaysState(ctx context.Context, chatID int64, id int64) error
	SetWaitingTimezoneState(ctx context.Context, chatID int64) error
	SetEditingState(ctx context.Context, chatID int64, id int64) error
	SetRescheduleState(ctx context.Context, chatID int64, id int64) error
	SetEditRepeatState(ctx context.Context, chatID int64, id int64) error

	// Pending data
	SetPendingText(ctx context.Context, chatID int64, text string) error
	GetPendingText(ctx context.Context, chatID int64) (string, error)
	SetPendingReminder(ctx context.Context, chatID int64, id int64) error

	// Session message
	SetSessionMessage(ctx context.Context, chatID int64, msgID int) error
	GetSessionMessage(ctx context.Context, chatID int64) (int, error)

	// Timezone
	GetTimezone(ctx context.Context, chatID int64) (string, error)
	SetTimezone(ctx context.Context, chatID int64, tz string) error

	// ID parsing (pure, no ctx needed)
	ParseEditingID(state string) (int64, bool)
	ParseRescheduleID(state string) (int64, bool)
	ParseEditRepeatID(state string) (int64, bool)
	ParseWeekdaysID(state string) (int64, bool)
	ParseCustomIntervalID(state string) (int64, bool)

	// Composite
	ResolveReminderID(ctx context.Context, chatID int64, state string, prefixes ...string) int64
	CleanupSessions(ctx context.Context) error
}

// Parserr defines time/interval parsing used by telegram handlers.
type Parserr interface {
	ParseTime(input string) (time.Time, error)
	ParseInterval(input string) (string, error)
}

// BotAPI defines the subset of telego.Bot methods used by handlers.
// *telego.Bot satisfies this interface implicitly.
type BotAPI interface {
	SendMessage(ctx context.Context, params *telego.SendMessageParams) (*telego.Message, error)
	EditMessageText(ctx context.Context, params *telego.EditMessageTextParams) (*telego.Message, error)
	DeleteMessage(ctx context.Context, params *telego.DeleteMessageParams) error
	AnswerCallbackQuery(ctx context.Context, params *telego.AnswerCallbackQueryParams) error
	GetChat(ctx context.Context, params *telego.GetChatParams) (*telego.ChatFullInfo, error)
}


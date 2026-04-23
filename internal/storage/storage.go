package storage

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("storage: entity not found")
	ErrAlreadyExists = errors.New("storage: entity already exists")
)

type Reminder struct {
	ID            int64
	ChatID        int64
	AuthorID      int64 // 0 = self-created; non-zero = created by a friend
	Text          string
	NotifyAt      time.Time
	Interval      string // Empty for one-time or custom duration
	Weekdays      int    // Bitmask: Mon=1, Tue=2, ..., Sun=64
	LastMessageID int
	CreatedAt     time.Time
}

type UserState struct {
	ChatID      int64
	State       string
	PendingText string
	Timezone    string
	UpdatedAt   time.Time
}

type User struct {
	ChatID    int64
	FirstName string
	LastName  string
	Username  string
	UpdatedAt time.Time
}

// FriendStatus represents the state of a friendship.
const (
	FriendStatusPending  = "pending"
	FriendStatusAccepted = "accepted"
)

// Friend represents a friendship link between two users.
type Friend struct {
	UserID    int64  // The user who sent the invite
	FriendID  int64  // The user who received the invite
	Status    string // "pending" or "accepted"
	CreatedAt time.Time
}

type ReminderRepository interface {
	Add(ctx context.Context, r *Reminder) error
	GetByChatID(ctx context.Context, chatID int64) ([]Reminder, error)
	GetByID(ctx context.Context, id int64) (*Reminder, error)
	Delete(ctx context.Context, chatID int64, id int64) error
	Update(ctx context.Context, r *Reminder) error

	GetDue(ctx context.Context, before time.Time) ([]Reminder, error)
	MarkAsNotified(ctx context.Context, id int64, nextNotifyAt time.Time) error
	DeleteByID(ctx context.Context, id int64) error

	// GetByAuthorAndTarget returns reminders created by authorID for targetChatID.
	GetByAuthorAndTarget(ctx context.Context, authorID, targetChatID int64) ([]Reminder, error)
	// GetFriendReminders returns all reminders where author_id != 0 for a given chat_id.
	GetFriendReminders(ctx context.Context, chatID int64) ([]Reminder, error)
	// ClearAuthor sets author_id = 0 for all reminders where authorID matches.
	ClearAuthor(ctx context.Context, authorID int64, targetChatID int64) error
}

type FriendRepository interface {
	// AddFriend creates a pending friend request (userID -> friendID).
	AddFriend(ctx context.Context, userID, friendID int64) error
	// AcceptFriend marks the request as accepted.
	AcceptFriend(ctx context.Context, userID, friendID int64) error
	// RemoveFriend removes the friendship in both directions.
	RemoveFriend(ctx context.Context, userID, friendID int64) error
	// GetFriends returns all accepted friends for a user.
	GetFriends(ctx context.Context, userID int64) ([]Friend, error)
	// GetPendingRequests returns pending friend requests sent TO userID.
	GetPendingRequests(ctx context.Context, userID int64) ([]Friend, error)
	// IsFriend checks if two users are accepted friends.
	IsFriend(ctx context.Context, userID, friendID int64) (bool, error)
	// HasPendingRequest checks if there is a pending request from userID to friendID.
	HasPendingRequest(ctx context.Context, userID, friendID int64) (bool, error)
}

type SessionRepository interface {
	SetState(ctx context.Context, chatID int64, state string) error
	GetState(ctx context.Context, chatID int64) (string, error)
	DeleteState(ctx context.Context, chatID int64) error

	SetPendingText(ctx context.Context, chatID int64, text string) error
	GetPendingText(ctx context.Context, chatID int64) (string, error)
	ClearPendingText(ctx context.Context, chatID int64) error

	SetSessionMessageID(ctx context.Context, chatID int64, messageID int) error
	GetSessionMessageID(ctx context.Context, chatID int64) (int, error)

	SetPendingReminderID(ctx context.Context, chatID int64, id int64) error
	GetPendingReminderID(ctx context.Context, chatID int64) (int64, error)

	SetTimezone(ctx context.Context, chatID int64, tz string) error
	GetTimezone(ctx context.Context, chatID int64) (string, error)

	Cleanup(ctx context.Context, olderThan time.Time) error
}

type UserRepository interface {
	Upsert(ctx context.Context, u *User) error
	Get(ctx context.Context, chatID int64) (*User, error)
}

type Storage interface {
	Reminders() ReminderRepository
	Sessions() SessionRepository
	Friends() FriendRepository
	Users() UserRepository
	Close() error
}

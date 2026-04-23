package core

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"reminder-bot/internal/storage"
)

// FriendService handles friend management logic.
type FriendService struct {
	friends   storage.FriendRepository
	reminders storage.ReminderRepository
	users     storage.UserRepository
	logger    *slog.Logger
}

func NewFriendService(s storage.Storage, logger *slog.Logger) *FriendService {
	return &FriendService{
		friends:   s.Friends(),
		reminders: s.Reminders(),
		users:     s.Users(),
		logger:    logger,
	}
}

// GenerateInvitePayload creates the deep-link payload for an invite.
// The link would be: t.me/<bot_username>?start=invite_<chatID>
func GenerateInvitePayload(chatID int64) string {
	return fmt.Sprintf("invite_%d", chatID)
}

// ParseInvitePayload extracts the inviter's chat ID from a /start payload.
// Returns (inviterChatID, true) on success.
func ParseInvitePayload(payload string) (int64, bool) {
	if !strings.HasPrefix(payload, "invite_") {
		return 0, false
	}
	idStr := strings.TrimPrefix(payload, "invite_")
	id, err := strconv.ParseInt(idStr, 10, 64)
	return id, err == nil
}

// SendFriendRequest creates a pending friend request from userID to friendID.
func (s *FriendService) SendFriendRequest(ctx context.Context, userID, friendID int64) error {
	if userID == friendID {
		return fmt.Errorf("cannot add yourself as a friend")
	}
	return s.friends.AddFriend(ctx, userID, friendID)
}

// AcceptFriendRequest accepts a pending request. The friendship becomes mutual.
func (s *FriendService) AcceptFriendRequest(ctx context.Context, fromUserID, toUserID int64) error {
	return s.friends.AcceptFriend(ctx, fromUserID, toUserID)
}

// RejectFriendRequest removes a pending request.
func (s *FriendService) RejectFriendRequest(ctx context.Context, fromUserID, toUserID int64) error {
	return s.friends.RemoveFriend(ctx, fromUserID, toUserID)
}

// RemoveFriend breaks the friendship and clears author from reminders in both directions.
func (s *FriendService) RemoveFriend(ctx context.Context, userID, friendID int64) error {
	// Clear author on reminders that userID created for friendID
	if err := s.reminders.ClearAuthor(ctx, userID, friendID); err != nil {
		s.logger.Error("failed to clear author on reminders", "author", userID, "target", friendID, "error", err)
	}
	// Clear author on reminders that friendID created for userID
	if err := s.reminders.ClearAuthor(ctx, friendID, userID); err != nil {
		s.logger.Error("failed to clear author on reminders", "author", friendID, "target", userID, "error", err)
	}
	return s.friends.RemoveFriend(ctx, userID, friendID)
}

// GetFriends returns all accepted friends for a user.
func (s *FriendService) GetFriends(ctx context.Context, userID int64) ([]storage.Friend, error) {
	return s.friends.GetFriends(ctx, userID)
}

// GetPendingRequests returns incoming friend requests for a user.
func (s *FriendService) GetPendingRequests(ctx context.Context, userID int64) ([]storage.Friend, error) {
	return s.friends.GetPendingRequests(ctx, userID)
}

// IsFriend checks if two users are friends.
func (s *FriendService) IsFriend(ctx context.Context, userID, friendID int64) (bool, error) {
	return s.friends.IsFriend(ctx, userID, friendID)
}

// HasPendingRequest checks if there is a pending request from userID to friendID.
func (s *FriendService) HasPendingRequest(ctx context.Context, userID, friendID int64) (bool, error) {
	return s.friends.HasPendingRequest(ctx, userID, friendID)
}

// UpsertUser saves or updates Telegram user profile information.
func (s *FriendService) UpsertUser(ctx context.Context, user *storage.User) error {
	return s.users.Upsert(ctx, user)
}

// GetUserInfo retrieves Telegram user profile information.
func (s *FriendService) GetUserInfo(ctx context.Context, chatID int64) (*storage.User, error) {
	return s.users.Get(ctx, chatID)
}

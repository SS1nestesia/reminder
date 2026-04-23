package core

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"reminder-bot/internal/storage"
)

// --- In-memory fakes for FriendRepository / UserRepository ---

type fakeFriendRepo struct {
	rows map[string]storage.Friend // key: "from:to"
}

func newFakeFriendRepo() *fakeFriendRepo {
	return &fakeFriendRepo{rows: make(map[string]storage.Friend)}
}

func fkey(a, b int64) string {
	return formatInt64(a) + ":" + formatInt64(b)
}

func formatInt64(v int64) string {
	// tiny, no strconv dep to keep this file self-contained
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func (f *fakeFriendRepo) AddFriend(_ context.Context, userID, friendID int64) error {
	k := fkey(userID, friendID)
	if _, exists := f.rows[k]; exists {
		return storage.ErrAlreadyExists
	}
	f.rows[k] = storage.Friend{UserID: userID, FriendID: friendID, Status: storage.FriendStatusPending, CreatedAt: time.Now()}
	return nil
}

func (f *fakeFriendRepo) AcceptFriend(_ context.Context, userID, friendID int64) error {
	fr, ok := f.rows[fkey(userID, friendID)]
	if !ok || fr.Status != storage.FriendStatusPending {
		return storage.ErrNotFound
	}
	fr.Status = storage.FriendStatusAccepted
	f.rows[fkey(userID, friendID)] = fr
	f.rows[fkey(friendID, userID)] = storage.Friend{UserID: friendID, FriendID: userID, Status: storage.FriendStatusAccepted, CreatedAt: time.Now()}
	return nil
}

func (f *fakeFriendRepo) RemoveFriend(_ context.Context, userID, friendID int64) error {
	delete(f.rows, fkey(userID, friendID))
	delete(f.rows, fkey(friendID, userID))
	return nil
}

func (f *fakeFriendRepo) GetFriends(_ context.Context, userID int64) ([]storage.Friend, error) {
	var out []storage.Friend
	for _, v := range f.rows {
		if v.UserID == userID && v.Status == storage.FriendStatusAccepted {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *fakeFriendRepo) GetPendingRequests(_ context.Context, userID int64) ([]storage.Friend, error) {
	var out []storage.Friend
	for _, v := range f.rows {
		if v.FriendID == userID && v.Status == storage.FriendStatusPending {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *fakeFriendRepo) IsFriend(_ context.Context, userID, friendID int64) (bool, error) {
	fr, ok := f.rows[fkey(userID, friendID)]
	return ok && fr.Status == storage.FriendStatusAccepted, nil
}

func (f *fakeFriendRepo) HasPendingRequest(_ context.Context, userID, friendID int64) (bool, error) {
	fr, ok := f.rows[fkey(userID, friendID)]
	return ok && fr.Status == storage.FriendStatusPending, nil
}

type fakeUserRepo struct{ users map[int64]storage.User }

func newFakeUserRepo() *fakeUserRepo { return &fakeUserRepo{users: make(map[int64]storage.User)} }

func (u *fakeUserRepo) Upsert(_ context.Context, user *storage.User) error {
	user.UpdatedAt = time.Now()
	u.users[user.ChatID] = *user
	return nil
}

func (u *fakeUserRepo) Get(_ context.Context, chatID int64) (*storage.User, error) {
	v, ok := u.users[chatID]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &v, nil
}

// fakeFriendStorage is a minimal storage.Storage that covers only the
// tables FriendService actually touches.
type fakeFriendStorage struct {
	reminders storage.ReminderRepository
	friends   storage.FriendRepository
	users     storage.UserRepository
}

func (s *fakeFriendStorage) Reminders() storage.ReminderRepository { return s.reminders }
func (s *fakeFriendStorage) Sessions() storage.SessionRepository   { return nil }
func (s *fakeFriendStorage) Friends() storage.FriendRepository     { return s.friends }
func (s *fakeFriendStorage) Users() storage.UserRepository         { return s.users }
func (s *fakeFriendStorage) Close() error                          { return nil }

func newFriendTestService() (*FriendService, *mockFullRepo, *fakeFriendRepo) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	reminders := newMockFullRepo()
	friends := newFakeFriendRepo()
	users := newFakeUserRepo()
	st := &fakeFriendStorage{reminders: reminders, friends: friends, users: users}
	return NewFriendService(st, logger), reminders, friends
}

// ==========================================
// Deep-link payload
// ==========================================

func TestGenerateAndParseInvitePayload_RoundTrip(t *testing.T) {
	chatID := int64(42)
	p := GenerateInvitePayload(chatID)
	if p != "invite_42" {
		t.Errorf("GenerateInvitePayload(42) = %q, want invite_42", p)
	}

	got, ok := ParseInvitePayload(p)
	if !ok || got != chatID {
		t.Errorf("ParseInvitePayload(%q) = (%d, %v), want (%d, true)", p, got, ok, chatID)
	}
}

func TestParseInvitePayload_Invalid(t *testing.T) {
	cases := []string{"", "foo", "invite_", "invite_abc", "hello_42"}
	for _, c := range cases {
		if _, ok := ParseInvitePayload(c); ok {
			t.Errorf("ParseInvitePayload(%q) should return ok=false", c)
		}
	}
}

// ==========================================
// Handshake: invite → accept → isFriend
// ==========================================

func TestFriendService_Handshake_HappyPath(t *testing.T) {
	svc, _, friends := newFriendTestService()
	ctx := context.Background()

	// User 1 invites user 2
	if err := svc.SendFriendRequest(ctx, 1, 2); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Pending visible to recipient
	pending, _ := svc.GetPendingRequests(ctx, 2)
	if len(pending) != 1 || pending[0].UserID != 1 {
		t.Fatalf("expected 1 pending request from user 1, got %+v", pending)
	}

	// Not friends yet
	if ok, _ := svc.IsFriend(ctx, 1, 2); ok {
		t.Error("users should not be friends before accept")
	}

	// Accept
	if err := svc.AcceptFriendRequest(ctx, 1, 2); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// Friendship is MUTUAL
	if ok, _ := svc.IsFriend(ctx, 1, 2); !ok {
		t.Error("1→2 should be friends after accept")
	}
	if ok, _ := svc.IsFriend(ctx, 2, 1); !ok {
		t.Error("2→1 should be friends after accept (mutuality)")
	}
	// Verify storage-level assertion
	if _, ok := friends.rows[fkey(2, 1)]; !ok {
		t.Error("reverse friendship row should exist")
	}
}

// ==========================================
// Reject
// ==========================================

func TestFriendService_Reject_RemovesPending(t *testing.T) {
	svc, _, _ := newFriendTestService()
	ctx := context.Background()

	_ = svc.SendFriendRequest(ctx, 1, 2)
	if err := svc.RejectFriendRequest(ctx, 1, 2); err != nil {
		t.Fatalf("reject: %v", err)
	}

	pending, _ := svc.GetPendingRequests(ctx, 2)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after reject, got %d", len(pending))
	}
}

// ==========================================
// Self-invite rejection
// ==========================================

func TestFriendService_CannotInviteSelf(t *testing.T) {
	svc, _, _ := newFriendTestService()
	if err := svc.SendFriendRequest(context.Background(), 42, 42); err == nil {
		t.Error("self-invite should return error")
	}
}

// ==========================================
// Duplicate invite
// ==========================================

func TestFriendService_DuplicateInvite_Rejected(t *testing.T) {
	svc, _, _ := newFriendTestService()
	ctx := context.Background()

	if err := svc.SendFriendRequest(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	err := svc.SendFriendRequest(ctx, 1, 2)
	if err == nil {
		t.Error("duplicate invite should fail")
	}
}

// ==========================================
// Author cleanup on unfriend (PROJECT_OVERVIEW Section C case 9)
// ==========================================

func TestFriendService_RemoveFriend_ClearsAuthorOnRemindersBothDirections(t *testing.T) {
	svc, reminders, _ := newFriendTestService()
	ctx := context.Background()

	// Establish friendship
	_ = svc.SendFriendRequest(ctx, 1, 2)
	_ = svc.AcceptFriendRequest(ctx, 1, 2)

	// User 1 creates a reminder for user 2
	_ = reminders.Add(ctx, &storage.Reminder{ChatID: 2, AuthorID: 1, Text: "By 1 for 2", NotifyAt: time.Now()})
	// User 2 creates a reminder for user 1
	_ = reminders.Add(ctx, &storage.Reminder{ChatID: 1, AuthorID: 2, Text: "By 2 for 1", NotifyAt: time.Now()})

	if err := svc.RemoveFriend(ctx, 1, 2); err != nil {
		t.Fatalf("RemoveFriend: %v", err)
	}

	// Both directions must have author_id cleared
	for _, r := range reminders.reminders {
		if r.AuthorID != 0 {
			t.Errorf("reminder %q still has AuthorID=%d after RemoveFriend", r.Text, r.AuthorID)
		}
	}

	// Friendship itself must be gone
	if ok, _ := svc.IsFriend(ctx, 1, 2); ok {
		t.Error("users should no longer be friends after RemoveFriend")
	}
}

// ==========================================
// Re-invite after removal (PROJECT_OVERVIEW Section D case 14)
// ==========================================

func TestFriendService_ReinviteAfterRemoval_Works(t *testing.T) {
	svc, _, _ := newFriendTestService()
	ctx := context.Background()

	// Complete cycle
	_ = svc.SendFriendRequest(ctx, 1, 2)
	_ = svc.AcceptFriendRequest(ctx, 1, 2)
	_ = svc.RemoveFriend(ctx, 1, 2)

	// Must be able to invite again without UNIQUE-constraint issues
	if err := svc.SendFriendRequest(ctx, 1, 2); err != nil {
		t.Fatalf("re-invite should succeed after removal, got %v", err)
	}
	has, _ := svc.HasPendingRequest(ctx, 1, 2)
	if !has {
		t.Error("expected a pending request after re-invite")
	}
}

// ==========================================
// Upsert / Get user profile cache
// ==========================================

func TestFriendService_UpsertAndGetUser(t *testing.T) {
	svc, _, _ := newFriendTestService()
	ctx := context.Background()

	if err := svc.UpsertUser(ctx, &storage.User{ChatID: 7, FirstName: "Ada", Username: "ada"}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetUserInfo(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.FirstName != "Ada" || got.Username != "ada" {
		t.Errorf("unexpected user data: %+v", got)
	}
}

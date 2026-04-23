package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// ==========================================
// Friends table
// ==========================================

func TestFriends_HandshakeAndMutuality(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	f := s.Friends()
	ctx := context.Background()

	if err := f.AddFriend(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	// Duplicate must fail with ErrAlreadyExists
	if err := f.AddFriend(ctx, 1, 2); !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("duplicate AddFriend: expected ErrAlreadyExists, got %v", err)
	}

	// Pending side observable to the recipient only
	pend, _ := f.GetPendingRequests(ctx, 2)
	if len(pend) != 1 {
		t.Fatalf("expected 1 pending for user 2, got %d", len(pend))
	}
	if pend2, _ := f.GetPendingRequests(ctx, 1); len(pend2) != 0 {
		t.Errorf("sender must not see their own request as pending, got %d", len(pend2))
	}

	// Accept establishes mutuality
	if err := f.AcceptFriend(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	got12, _ := f.IsFriend(ctx, 1, 2)
	got21, _ := f.IsFriend(ctx, 2, 1)
	if !got12 || !got21 {
		t.Errorf("expected mutual friendship, got 1→2=%v, 2→1=%v", got12, got21)
	}
}

// TestFriends_RemoveAndReinvite is a regression test for
// PROJECT_OVERVIEW §D case 14: re-inviting after a removal must not
// collide with the old (deleted) row.
func TestFriends_RemoveAndReinvite(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	f := s.Friends()
	ctx := context.Background()

	_ = f.AddFriend(ctx, 1, 2)
	_ = f.AcceptFriend(ctx, 1, 2)
	if err := f.RemoveFriend(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}

	// Re-invite must succeed without UNIQUE-constraint violation
	if err := f.AddFriend(ctx, 1, 2); err != nil {
		t.Errorf("re-invite after removal failed: %v", err)
	}
}

func TestFriends_AcceptMissing_ReturnsErrNotFound(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	f := s.Friends()

	if err := f.AcceptFriend(context.Background(), 1, 2); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound accepting unknown request, got %v", err)
	}
}

// ==========================================
// Users table
// ==========================================

func TestUsers_UpsertRoundTrip(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	u := s.Users()
	ctx := context.Background()

	// First insert
	if err := u.Upsert(ctx, &User{ChatID: 10, FirstName: "Ada", Username: "ada"}); err != nil {
		t.Fatal(err)
	}
	got, _ := u.Get(ctx, 10)
	if got.FirstName != "Ada" || got.Username != "ada" {
		t.Errorf("after upsert: %+v", got)
	}

	// Upsert updates existing row
	time.Sleep(1 * time.Millisecond)
	if err := u.Upsert(ctx, &User{ChatID: 10, FirstName: "Ada Lovelace", Username: "ada"}); err != nil {
		t.Fatal(err)
	}
	got, _ = u.Get(ctx, 10)
	if got.FirstName != "Ada Lovelace" {
		t.Errorf("expected updated name, got %q", got.FirstName)
	}
}

func TestUsers_Get_NotFound(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	if _, err := s.Users().Get(context.Background(), 99999); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ==========================================
// Reminder / Friend scope isolation
// ==========================================

// TestReminders_GetByChatID_ExcludesFriendReminders verifies that the
// owner's "own list" never contains friend-authored reminders (those
// must flow through GetFriendReminders instead).
func TestReminders_GetByChatID_ExcludesFriendReminders(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	r := s.Reminders()
	ctx := context.Background()

	_ = r.Add(ctx, &Reminder{ChatID: 1, Text: "self", NotifyAt: time.Now()})
	_ = r.Add(ctx, &Reminder{ChatID: 1, AuthorID: 99, Text: "by friend", NotifyAt: time.Now()})

	own, _ := r.GetByChatID(ctx, 1)
	if len(own) != 1 || own[0].Text != "self" {
		t.Errorf("GetByChatID should return only self reminders, got %+v", own)
	}

	friend, _ := r.GetFriendReminders(ctx, 1)
	if len(friend) != 1 || friend[0].Text != "by friend" {
		t.Errorf("GetFriendReminders should return only friend-authored, got %+v", friend)
	}
}

func TestReminders_ClearAuthor_ScopedToPair(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	r := s.Reminders()
	ctx := context.Background()

	// authorA → chatB, authorA → chatC, authorX → chatB
	_ = r.Add(ctx, &Reminder{ChatID: 2, AuthorID: 1, Text: "a→b", NotifyAt: time.Now()})
	_ = r.Add(ctx, &Reminder{ChatID: 3, AuthorID: 1, Text: "a→c", NotifyAt: time.Now()})
	_ = r.Add(ctx, &Reminder{ChatID: 2, AuthorID: 99, Text: "x→b", NotifyAt: time.Now()})

	if err := r.ClearAuthor(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}

	// Only the a→b row's author should be cleared
	rems2, _ := r.GetFriendReminders(ctx, 2)
	for _, rem := range rems2 {
		if rem.Text == "a→b" {
			t.Errorf("a→b author must be cleared, got AuthorID=%d", rem.AuthorID)
		}
		if rem.Text == "x→b" && rem.AuthorID != 99 {
			t.Errorf("x→b must be untouched, got AuthorID=%d", rem.AuthorID)
		}
	}

	// a→c must be untouched
	rems3, _ := r.GetFriendReminders(ctx, 3)
	if len(rems3) != 1 || rems3[0].AuthorID != 1 {
		t.Errorf("ClearAuthor should not affect other chatIDs, got %+v", rems3)
	}
}

// ==========================================
// GetByAuthorAndTarget
// ==========================================

func TestReminders_GetByAuthorAndTarget(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	r := s.Reminders()
	ctx := context.Background()

	_ = r.Add(ctx, &Reminder{ChatID: 2, AuthorID: 1, Text: "match", NotifyAt: time.Now()})
	_ = r.Add(ctx, &Reminder{ChatID: 3, AuthorID: 1, Text: "wrong chat", NotifyAt: time.Now()})
	_ = r.Add(ctx, &Reminder{ChatID: 2, AuthorID: 9, Text: "wrong author", NotifyAt: time.Now()})

	got, _ := r.GetByAuthorAndTarget(ctx, 1, 2)
	if len(got) != 1 || got[0].Text != "match" {
		t.Errorf("expected only the (1→2) match, got %+v", got)
	}
}

// ==========================================
// Concurrency — write safety of SQLite repos
// ==========================================

// TestReminders_ConcurrentAdd_NoCorruption spawns N goroutines inserting
// reminders in parallel and then asserts every row is retrievable and
// IDs are unique.
func TestReminders_ConcurrentAdd_NoCorruption(t *testing.T) {
	s, cleanup := newTestStorage(t)
	defer cleanup()
	r := s.Reminders()
	ctx := context.Background()

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	ids := make([]int64, N)

	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			rem := &Reminder{ChatID: int64(i%5 + 1), Text: "concurrent", NotifyAt: time.Now()}
			if err := r.Add(ctx, rem); err != nil {
				t.Errorf("concurrent add #%d failed: %v", i, err)
				return
			}
			ids[i] = rem.ID
		}(i)
	}
	wg.Wait()

	seen := make(map[int64]bool, N)
	for _, id := range ids {
		if id == 0 {
			t.Error("id=0 means LastInsertId was silently skipped")
		}
		if seen[id] {
			t.Errorf("duplicate ID generated: %d", id)
		}
		seen[id] = true
		if _, err := r.GetByID(ctx, id); err != nil {
			t.Errorf("row with id %d unreachable: %v", id, err)
		}
	}
}

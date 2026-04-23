package telegram

import (
	"log/slog"
	"testing"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

func TestListHandlers(t *testing.T) {
	b := newMockBot()
	s := newMockService()
	st := newMockState()
	p := newMockParser()
	logger := slog.Default()

	h := NewHandlers(b, s, &mockFriendService{}, p, st, "testbot", logger)
	ctx := &th.Context{}

	cq := telego.CallbackQuery{
		Message: &telego.Message{
			Chat:      telego.Chat{ID: 1},
			MessageID: 100,
		},
	}

	t.Run("handleListReminders", func(t *testing.T) {
		query := cq
		query.Data = "list_reminders"
		err := h.list.handleListReminders(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleView", func(t *testing.T) {
		query := cq
		query.Data = "view:42"
		err := h.list.handleView(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleDone", func(t *testing.T) {
		query := cq
		query.Data = "done:42"
		err := h.list.handleDone(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleConfirmDelete", func(t *testing.T) {
		query := cq
		query.Data = "confirm_delete:42"
		err := h.list.handleConfirmDelete(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleDelete", func(t *testing.T) {
		query := cq
		query.Data = "delete:42"
		err := h.list.handleDelete(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

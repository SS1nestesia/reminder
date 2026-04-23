package telegram

import (
	"log/slog"
	"testing"
	"time"

	"reminder-bot/internal/storage"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

func TestEditorHandlers(t *testing.T) {
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

	t.Run("handleEditText", func(t *testing.T) {
		query := cq
		query.Data = "edit_text:42"
		err := h.editor.handleEditText(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleEditTime", func(t *testing.T) {
		query := cq
		query.Data = "edit_time:42"
		err := h.editor.handleEditTime(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleEditRepeat", func(t *testing.T) {
		query := cq
		query.Data = "edit_repeat:42"
		err := h.editor.handleEditRepeat(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleReschedule", func(t *testing.T) {
		query := cq
		query.Data = "reschedule:42"
		err := h.editor.handleReschedule(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleSnoozeMenu", func(t *testing.T) {
		query := cq
		query.Data = "snooze_menu:42"
		err := h.editor.handleSnoozeMenu(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleSnoozeBack", func(t *testing.T) {
		query := cq
		query.Data = "snooze_back:42"

		s.mu.Lock()
		s.reminders[1] = append(s.reminders[1], storage.Reminder{ID: 42, ChatID: 1, Text: "T", NotifyAt: time.Now()})
		s.mu.Unlock()

		err := h.editor.handleSnoozeBack(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleTextEdit", func(t *testing.T) {
		err := h.editor.handleTextEdit(ctx, 1, 100, "editing:42", "new text")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleSnoozeApply", func(t *testing.T) {
		query := cq
		query.Data = "snooze:10m:42"

		// Mock target reminder so format works
		s.mu.Lock()
		s.reminders[1] = []storage.Reminder{{ID: 42, ChatID: 1, Text: "T", NotifyAt: time.Now()}}
		s.mu.Unlock()

		err := h.editor.handleSnoozeApply(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

package telegram

import (
	"context"
	"log/slog"
	"testing"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

func TestCreatorHandlers(t *testing.T) {
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

	t.Run("handleAddReminder", func(t *testing.T) {
		query := cq
		query.Data = "add_reminder"
		err := h.creator.handleAddReminder(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleQuickTime", func(t *testing.T) {
		query := cq
		query.Data = "quick:10m"
		err := h.creator.handleQuickTime(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleTextTime", func(t *testing.T) {
		err := h.creator.handleTextTime(ctx, 1, 100, "waiting_time", "12:00")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleTextNew", func(t *testing.T) {
		err := h.creator.handleTextNew(ctx, 1, 100, "buy milk")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleCancel", func(t *testing.T) {
		query := cq
		query.Data = "cancel"
		err := h.creator.handleCancel(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleRepeat", func(t *testing.T) {
		query := cq
		query.Data = "repeat:24h"
		err := h.creator.handleRepeat(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleWeekday_Toggle", func(t *testing.T) {
		query := cq
		query.Data = "wd:1"
		st.SetWaitingWeekdaysState(context.Background(), 1, 42)
		err := h.creator.handleWeekday(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleWeekday_Done", func(t *testing.T) {
		query := cq
		query.Data = "wd:done"
		
		msg := &telego.Message{
			Chat:      telego.Chat{ID: 1},
			MessageID: 100,
			ReplyMarkup: WeekdaysKeyboard(1<<1).(*telego.InlineKeyboardMarkup),
		}
		query.Message = msg
		
		st.SetWaitingWeekdaysState(context.Background(), 1, 42)
		err := h.creator.handleWeekday(ctx, query)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleWeekday_DoneEmpty", func(t *testing.T) {
		query := cq
		query.Data = "wd:done"
		msg := &telego.Message{
			Chat:      telego.Chat{ID: 1},
			MessageID: 100,
			ReplyMarkup: WeekdaysKeyboard(0).(*telego.InlineKeyboardMarkup),
		}
		query.Message = msg
		
		st.SetWaitingWeekdaysState(context.Background(), 1, 42)
		// Should return an answer callback with error
		_ = h.creator.handleWeekday(ctx, query)
	})
}

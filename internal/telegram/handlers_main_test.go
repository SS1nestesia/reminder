package telegram

import (
	"log/slog"
	"testing"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

func TestMainHandlers(t *testing.T) {
	b := newMockBot()
	s := newMockService()
	st := newMockState()
	p := newMockParser()
	logger := slog.Default()

	h := NewHandlers(b, s, &mockFriendService{}, p, st, "testbot", logger)
	ctx := &th.Context{}

	t.Run("handleStart", func(t *testing.T) {
		msg := telego.Message{
			Chat: telego.Chat{ID: 1},
			Text: "/start",
		}
		err := h.handleStart(ctx, msg)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleBackToMenu", func(t *testing.T) {
		cq := telego.CallbackQuery{
			Message: &telego.Message{
				Chat:      telego.Chat{ID: 1},
				MessageID: 100,
			},
			Data: "back_to_menu",
		}
		err := h.handleBackToMenu(ctx, cq)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleSetupTimezone", func(t *testing.T) {
		cq := telego.CallbackQuery{
			Message: &telego.Message{
				Chat:      telego.Chat{ID: 1},
				MessageID: 100,
			},
			Data: "setup_timezone",
		}
		err := h.handleSetupTimezone(ctx, cq)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handleTimezoneChoice", func(t *testing.T) {
		cq := telego.CallbackQuery{
			Message: &telego.Message{
				Chat:      telego.Chat{ID: 1},
				MessageID: 100,
			},
			Data: "tz:Europe/Moscow",
		}
		err := h.handleTimezoneChoice(ctx, cq)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}



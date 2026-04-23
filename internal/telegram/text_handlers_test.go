package telegram

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

func TestHandleTextMessage(t *testing.T) {
	b := newMockBot()
	s := newMockService()
	st := newMockState()
	p := newMockParser()
	logger := slog.Default()

	h := NewHandlers(b, s, &mockFriendService{}, p, st, "testbot", logger)

	// th.Context inside telego has a fallback if ctx is nil/empty.
	ctx := &th.Context{}

	callHandler := func(chatID int64, text string) error {
		msg := telego.Message{
			Chat: telego.Chat{ID: chatID},
			Text: text,
		}
		return h.handleTextMessage(ctx, msg)
	}

	// 1. Initial State (No State)
	_ = st.SetTimezone(context.Background(), 1, "Europe/Moscow")
	err := callHandler(1, "hello")
	if err != nil {
		t.Fatalf("unexpected error for no state: %v", err)
	}
	if len(b.sentMessages) == 0 {
		t.Errorf("expected bot to send message for no state")
	} else if !strings.Contains(b.sentMessages[0].Text, "Выберите действие") {
		t.Errorf("expected Main Menu, got %s", b.sentMessages[0].Text)
	}
	b.sentMessages = nil

	// 2. Custom Interval State
	_ = st.SetState(context.Background(), 1, "custom:123")
	err = callHandler(1, "5h")
	if err != nil {
		t.Fatalf("unexpected error custom interval: %v", err)
	}
	_ = st.ClearState(context.Background(), 1)

	// 3. Editing Prefix State
	_ = st.SetState(context.Background(), 1, "editing:123")
	err = callHandler(1, "new text")
	if err != nil {
		t.Fatalf("unexpected error editing text: %v", err)
	}
	_ = st.ClearState(context.Background(), 1)

	// 4. Waiting Reminder Time State
	_ = st.SetState(context.Background(), 1, "waiting_time")
	err = callHandler(1, "12:00")
	if err != nil {
		t.Fatalf("unexpected error waiting time: %v", err)
	}
	_ = st.ClearState(context.Background(), 1)

	// 5. Waiting Reminder Text State
	_ = st.SetState(context.Background(), 1, "waiting_text")
	err = callHandler(1, "buy milk")
	if err != nil {
		t.Fatalf("unexpected error waiting text: %v", err)
	}
	_ = st.ClearState(context.Background(), 1)

	// 6. Waiting Timezone State
	_ = st.SetState(context.Background(), 1, "waiting_timezone")
	err = callHandler(1, "Europe/London")
	if err != nil {
		t.Fatalf("unexpected error waiting timezone: %v", err)
	}
	_ = st.ClearState(context.Background(), 1)

	// 7. Unknown State -> fallback to main menu
	_ = st.SetState(context.Background(), 1, "unknown_weird_state")
	err = callHandler(1, "xyz")
	if err != nil {
		t.Fatalf("unexpected error unknown state: %v", err)
	}
	if len(b.sentMessages) == 0 {
		t.Errorf("expected fallback to main menu")
	}
}

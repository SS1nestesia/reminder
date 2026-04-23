package telegram

import (
	"context"
	"log/slog"
	"testing"
)

func TestReportErrorAndSuccess(t *testing.T) {
	b := newMockBot()
	s := newMockService()
	st := newMockState()
	p := newMockParser()
	logger := slog.Default()

	h := NewHandlers(b, s, &mockFriendService{}, p, st, "testbot", logger)

	t.Run("reportError", func(t *testing.T) {
		err := h.reportError(context.Background(), 1, 100, "oops", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(b.editMessages) == 0 {
			t.Fatalf("expected edit message to be called")
		}
		if b.editMessages[len(b.editMessages)-1].Text != "❌ oops" {
			t.Errorf("expected text '❌ oops', got %s", b.editMessages[0].Text)
		}
	})

	t.Run("reportSuccess", func(t *testing.T) {
		err := h.reportSuccess(context.Background(), 1, 100, "yay", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(b.editMessages) == 0 {
			t.Fatalf("expected edit message to be called")
		}
		if b.editMessages[len(b.editMessages)-1].Text != "✅ yay" {
			t.Errorf("expected text '✅ yay', got %s", b.editMessages[len(b.editMessages)-1].Text)
		}
	})
}

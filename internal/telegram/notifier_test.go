package telegram

import (
	"context"
	"testing"

	"reminder-bot/internal/storage"
)

func TestTelegramNotifier_Notify(t *testing.T) {
	b := newMockBot()
	notifier := NewTelegramNotifier(b)

	r := storage.Reminder{
		ID:     42,
		ChatID: 100,
		Text:   "Test Notification",
	}

	msgID, err := notifier.Notify(context.Background(), r)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if msgID != 100 { // mockBot returns 100
		t.Errorf("expected message ID 100, got %d", msgID)
	}

	if len(b.sentMessages) == 0 {
		t.Fatalf("expected message to be sent")
	}

	sentList := b.sentMessages[0]
	if sentList.ChatID.ID != 100 {
		t.Errorf("expected chatID 100, got %d", sentList.ChatID.ID)
	}
}

func TestTelegramNotifier_DeleteMessage(t *testing.T) {
	b := newMockBot()
	notifier := NewTelegramNotifier(b)

	err := notifier.DeleteMessage(context.Background(), 100, 42)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(b.delMessages) == 0 {
		t.Fatalf("expected message to be deleted")
	}

	delParam := b.delMessages[0]
	if delParam.ChatID.ID != 100 || delParam.MessageID != 42 {
		t.Errorf("expected chat 100 msg 42, got %v", delParam)
	}
}

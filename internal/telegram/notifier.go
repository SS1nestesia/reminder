package telegram

import (
	"context"
	"fmt"
	"html"

	"reminder-bot/internal/storage"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// TelegramNotifier implements the Notifier interface using the Telegram Bot API.
// It is used by the Scheduler to send reminder notifications independently of the Handlers.
type TelegramNotifier struct {
	bot BotAPI
}

func NewTelegramNotifier(bot BotAPI) *TelegramNotifier {
	return &TelegramNotifier{bot: bot}
}

func (n *TelegramNotifier) Notify(ctx context.Context, r storage.Reminder) (int, error) {
	text := fmt.Sprintf("⏰ <b>НАПОМИНАНИЕ!</b>\n\n%s\n\nНажми кнопку ниже 👇", html.EscapeString(r.Text))
	msg, err := n.bot.SendMessage(ctx, tu.Message(tu.ID(r.ChatID), text).
		WithParseMode(telego.ModeHTML).
		WithReplyMarkup(NotificationKeyboard(r.ID)))
	if err != nil {
		return 0, err
	}
	return msg.MessageID, nil
}

func (n *TelegramNotifier) DeleteMessage(ctx context.Context, chatID int64, msgID int) error {
	return n.bot.DeleteMessage(ctx, &telego.DeleteMessageParams{
		ChatID:    tu.ID(chatID),
		MessageID: msgID,
	})
}

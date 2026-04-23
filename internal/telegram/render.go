package telegram

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"reminder-bot/internal/core"
	"reminder-bot/internal/storage"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// --- Messaging Helpers ---

func (h *Handlers) send(ctx context.Context, chatID int64, text string, markup telego.ReplyMarkup) error {
	params := tu.Message(tu.ID(chatID), text).WithParseMode(telego.ModeHTML)
	if markup != nil {
		params.WithReplyMarkup(markup)
	}
	_, err := h.bot.SendMessage(ctx, params)
	return err
}

func (h *Handlers) edit(ctx context.Context, chatID int64, msgID int, text string, markup *telego.InlineKeyboardMarkup) error {
	if msgID == 0 {
		return h.send(ctx, chatID, text, markup)
	}
	params := tu.EditMessageText(tu.ID(chatID), msgID, text).WithParseMode(telego.ModeHTML)
	if markup != nil {
		params.WithReplyMarkup(markup)
	}
	_, err := h.bot.EditMessageText(ctx, params)
	if err != nil {
		h.logger.Warn("edit failed, falling back to send", "error", err)
		return h.send(ctx, chatID, text, markup)
	}
	return nil
}

func (h *Handlers) reportError(ctx context.Context, chatID int64, msgID int, text string, kb telego.ReplyMarkup) error {
	h.logger.Error("handler error", "chat_id", chatID, "message", text)
	var markup *telego.InlineKeyboardMarkup
	if kb != nil {
		markup, _ = kb.(*telego.InlineKeyboardMarkup)
	}
	if markup == nil {
		markup = BackToMenuKeyboard().(*telego.InlineKeyboardMarkup)
	}
	return h.edit(ctx, chatID, msgID, "❌ "+text, markup)
}

func (h *Handlers) reportSuccess(ctx context.Context, chatID int64, msgID int, text string, kb telego.ReplyMarkup) error {
	var markup *telego.InlineKeyboardMarkup
	if kb != nil {
		markup, _ = kb.(*telego.InlineKeyboardMarkup)
	}
	if markup == nil {
		markup = BackToMenuKeyboard().(*telego.InlineKeyboardMarkup)
	}
	return h.edit(ctx, chatID, msgID, "✅ "+text, markup)
}

func (h *Handlers) answer(ctx context.Context, queryID string) {
	if err := h.bot.AnswerCallbackQuery(ctx, tu.CallbackQuery(queryID)); err != nil {
		h.logger.Error("failed to answer callback query", "id", queryID, "error", err)
	}
}

// --- View Rendering ---

func (h *Handlers) showReminderDetail(ctx context.Context, chatID int64, msgID int, id int64) error {
	target, err := h.service.GetReminder(ctx, id)
	if err != nil || (target.ChatID != chatID && target.AuthorID != chatID) {
		userLoc := h.service.GetUserLocation(ctx, chatID)
		rems, _ := h.service.GetReminders(ctx, chatID)
		return h.edit(ctx, chatID, msgID, MsgNotFound, ListKeyboard(rems, userLoc).(*telego.InlineKeyboardMarkup))
	}

	userLoc := h.service.GetUserLocation(ctx, chatID)
	authorLine := ""
	if target.AuthorID != 0 {
		authorLine = "\n👤 От друга"
	}
	text := fmt.Sprintf("📝 <b>%s</b>\n\n⏰ %s\n🔄 Повтор: <b>%s</b>%s",
		html.EscapeString(target.Text),
		target.NotifyAt.In(userLoc).Format("02.01.2006 в 15:04"),
		core.FormatRecurrence(*target),
		authorLine,
	)
	return h.edit(ctx, chatID, msgID, text, ReminderActionsKeyboard(id).(*telego.InlineKeyboardMarkup))
}

func (h *Handlers) buildListText(ctx context.Context, chatID int64, rems []storage.Reminder) string {
	if len(rems) == 0 {
		return MsgEmptyList
	}
	userLoc := h.service.GetUserLocation(ctx, chatID)
	var sb strings.Builder
	sb.WriteString("📋 <b>Ваши напоминания</b>\n\n")
	for i, r := range rems {
		timeStr := r.NotifyAt.In(userLoc).Format("15:04 02.01")
		sb.WriteString(fmt.Sprintf("🔸 %d. <b>%s</b>\n   ⏰ %s\n   🔄 Повтор: %s\n\n",
			i+1, html.EscapeString(r.Text), timeStr, core.FormatRecurrence(r)))
	}
	return sb.String()
}

// --- Keyboard Builders ---

func MainMenuKeyboard(tz string, pendingCount int) telego.ReplyMarkup {
	rows := [][]telego.InlineKeyboardButton{}

	if tz == "" {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🌐 Настроить время (обязательно)").WithCallbackData("setup_timezone"),
		))
	} else {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(fmt.Sprintf("🌐 Время: %s", core.FormatTimezone(tz))).WithCallbackData("setup_timezone"),
		))
	}

	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("➕ Добавить").WithCallbackData("add_reminder"),
		tu.InlineKeyboardButton("📋 Мои напоминания").WithCallbackData("list_reminders"),
	))

	friendsText := "👥 Друзья"
	if pendingCount > 0 {
		friendsText = fmt.Sprintf("👥 Друзья (%d)", pendingCount)
	}

	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(friendsText).WithCallbackData(CBFriendsMenu),
		tu.InlineKeyboardButton("📬 От друзей").WithCallbackData(CBFriendReminders),
	))

	return tu.InlineKeyboard(rows...)
}

func BackToMenuKeyboard() telego.ReplyMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("⬅️ В меню").WithCallbackData("back_to_menu")),
	)
}

func TimezoneQuickKeyboard() telego.ReplyMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Москва (+3)").WithCallbackData("tz:Europe/Moscow"),
			tu.InlineKeyboardButton("Киев (+2)").WithCallbackData("tz:Europe/Kiev"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Екатеринбург (+5)").WithCallbackData("tz:Asia/Yekaterinburg"),
			tu.InlineKeyboardButton("Новосибирск (+7)").WithCallbackData("tz:Asia/Novosibirsk"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("UTC (+0)").WithCallbackData("tz:Etc/GMT"),
			tu.InlineKeyboardButton("❌ Отменить").WithCallbackData("cancel"),
		),
	)
}

func CancelKeyboard() telego.ReplyMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("❌ Отменить").WithCallbackData("cancel")),
	)
}

func CancelEditKeyboard(id int64) telego.ReplyMarkup {
	idStr := strconv.FormatInt(id, 10)
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("❌ Отменить").WithCallbackData("view:" + idStr)),
	)
}

func QuickTimeEditKeyboard(id int64) telego.ReplyMarkup {
	idStr := strconv.FormatInt(id, 10)
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("10 мин").WithCallbackData("quick:10m"),
			tu.InlineKeyboardButton("30 мин").WithCallbackData("quick:30m"),
			tu.InlineKeyboardButton("1 час").WithCallbackData("quick:1h"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("2 часа").WithCallbackData("quick:2h"),
			tu.InlineKeyboardButton("❌ Отменить").WithCallbackData("view:"+idStr),
		),
	)
}

func RecurrenceEditKeyboard(id int64) telego.ReplyMarkup {
	idStr := strconv.FormatInt(id, 10)
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ Без повтора").WithCallbackData("repeat:none"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📅 Каждый день").WithCallbackData("repeat:24h"),
			tu.InlineKeyboardButton("🗓 Каждую неделю").WithCallbackData("repeat:168h"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📅 По дням недели").WithCallbackData("repeat:weekdays"),
			tu.InlineKeyboardButton("⚙️ Свой вариант").WithCallbackData("repeat:custom"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ Отменить").WithCallbackData("view:"+idStr),
		),
	)
}

func ConfirmDeleteKeyboard(id int64) telego.ReplyMarkup {
	idStr := strconv.FormatInt(id, 10)
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ Да, удалить").WithCallbackData("delete:"+idStr),
			tu.InlineKeyboardButton("❌ Нет").WithCallbackData("view:"+idStr),
		),
	)
}

func QuickTimeKeyboard() telego.ReplyMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("10 мин").WithCallbackData("quick:10m"),
			tu.InlineKeyboardButton("30 мин").WithCallbackData("quick:30m"),
			tu.InlineKeyboardButton("1 час").WithCallbackData("quick:1h"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("2 часа").WithCallbackData("quick:2h"),
			tu.InlineKeyboardButton("❌ Отменить").WithCallbackData("cancel"),
		),
	)
}

func RecurrenceKeyboard() telego.ReplyMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ Без повтора").WithCallbackData("repeat:none"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📅 Каждый день").WithCallbackData("repeat:24h"),
			tu.InlineKeyboardButton("🗓 Каждую неделю").WithCallbackData("repeat:168h"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📅 По дням недели").WithCallbackData("repeat:weekdays"),
			tu.InlineKeyboardButton("⚙️ Свой вариант").WithCallbackData("repeat:custom"),
		),
	)
}

func WeekdaysKeyboard(mask int) telego.ReplyMarkup {
	names := []string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"}
	var rows [][]telego.InlineKeyboardButton

	var row1 []telego.InlineKeyboardButton
	for i := 0; i < 4; i++ {
		text := names[i]
		if (mask & (1 << uint(i))) != 0 {
			text = "✅ " + text
		}
		row1 = append(row1, tu.InlineKeyboardButton(text).WithCallbackData(fmt.Sprintf("wd:%d", i)))
	}
	rows = append(rows, row1)

	var row2 []telego.InlineKeyboardButton
	for i := 4; i < 7; i++ {
		text := names[i]
		if (mask & (1 << uint(i))) != 0 {
			text = "✅ " + text
		}
		row2 = append(row2, tu.InlineKeyboardButton(text).WithCallbackData(fmt.Sprintf("wd:%d", i)))
	}
	rows = append(rows, row2)

	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("✅ Готово").WithCallbackData("wd:done"),
	))

	return tu.InlineKeyboard(rows...)
}

func ListKeyboard(reminders []storage.Reminder, userLoc *time.Location) telego.ReplyMarkup {
	if len(reminders) == 0 {
		return tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("➕ Добавить напоминание").WithCallbackData("add_reminder"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("⬅️ В меню").WithCallbackData("back_to_menu"),
			),
		)
	}

	if userLoc == nil {
		userLoc = core.DefaultLoc
	}

	var rows [][]telego.InlineKeyboardButton
	for _, r := range reminders {
		timeStr := r.NotifyAt.In(userLoc).Format("02.01 15:04")
		btnText := r.Text + " · " + timeStr
		if utf8.RuneCountInString(btnText) > 55 {
			btnText = string([]rune(btnText)[:52]) + "..."
		}
		idStr := strconv.FormatInt(r.ID, 10)
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(btnText).WithCallbackData("view:"+idStr),
		))
	}

	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("➕ Добавить напоминание").WithCallbackData("add_reminder"),
		tu.InlineKeyboardButton("⬅️ В меню").WithCallbackData("back_to_menu"),
	))

	return tu.InlineKeyboard(rows...)
}

func NotificationKeyboard(id int64) telego.ReplyMarkup {
	idStr := strconv.FormatInt(id, 10)
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ Сделано").WithCallbackData("done:"+idStr),
			tu.InlineKeyboardButton("💤 Отложить...").WithCallbackData("snooze_menu:"+idStr),
			tu.InlineKeyboardButton("⏰ Другое время").WithCallbackData("reschedule:"+idStr),
		),
	)
}

func SnoozeOptionsKeyboard(id int64) telego.ReplyMarkup {
	idStr := strconv.FormatInt(id, 10)
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("5 минут").WithCallbackData("snooze:5m:"+idStr),
			tu.InlineKeyboardButton("30 минут").WithCallbackData("snooze:30m:"+idStr),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("1 час").WithCallbackData("snooze:1h:"+idStr),
			tu.InlineKeyboardButton("Завтра").WithCallbackData("snooze:24h:"+idStr),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ Назад").WithCallbackData("snooze_back:"+idStr),
		),
	)
}

func ReminderActionsKeyboard(id int64) telego.ReplyMarkup {
	idStr := strconv.FormatInt(id, 10)
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✏️ Текст").WithCallbackData("edit_text:"+idStr),
			tu.InlineKeyboardButton("⏰ Время").WithCallbackData("edit_time:"+idStr),
			tu.InlineKeyboardButton("🔄 Повтор").WithCallbackData("edit_repeat:"+idStr),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🗑 Удалить").WithCallbackData("confirm_delete:"+idStr),
			tu.InlineKeyboardButton("⬅️ К списку").WithCallbackData("list_reminders"),
		),
	)
}

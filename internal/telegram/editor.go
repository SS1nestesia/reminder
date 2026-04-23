package telegram

import (
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"reminder-bot/internal/core"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

type EditorHandlers struct {
	bot     BotAPI
	service ReminderServicer
	parser  Parserr
	state   StateManagerr
	logger  *slog.Logger
	common  *Handlers
}

func (h *EditorHandlers) handleEditText(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)

	id, ok := callbackID(query.Data, "edit_text:")
	if !ok {
		return nil
	}

	if err := h.state.SetEditingState(ctx.Context(), chatID, id); err != nil {
		h.logger.Error("failed to set state", "error", err)
	}
	h.state.SetSessionMessage(ctx.Context(), chatID, msgID)

	return h.common.edit(ctx.Context(), chatID, msgID, "✏️ <b>Отправьте новый текст напоминания:</b>", CancelEditKeyboard(id).(*telego.InlineKeyboardMarkup))
}

func (h *EditorHandlers) handleEditTime(ctx *th.Context, query telego.CallbackQuery) error {
	return h.startReschedule(ctx, query, "edit_time:")
}

func (h *EditorHandlers) handleReschedule(ctx *th.Context, query telego.CallbackQuery) error {
	return h.startReschedule(ctx, query, "reschedule:")
}

// startReschedule is the shared logic for handleEditTime and handleReschedule.
func (h *EditorHandlers) startReschedule(ctx *th.Context, query telego.CallbackQuery, prefix string) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)

	id, ok := callbackID(query.Data, prefix)
	if !ok {
		return nil
	}

	if err := h.state.SetRescheduleState(ctx.Context(), chatID, id); err != nil {
		h.logger.Error("failed to set state", "error", err)
	}
	h.state.SetSessionMessage(ctx.Context(), chatID, msgID)

	return h.common.edit(ctx.Context(), chatID, msgID, "⏰ <b>Укажите новое время напоминания:</b>", QuickTimeEditKeyboard(id).(*telego.InlineKeyboardMarkup))
}

func (h *EditorHandlers) handleEditRepeat(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)

	id, ok := callbackID(query.Data, "edit_repeat:")
	if !ok {
		return nil
	}

	if err := h.state.SetEditRepeatState(ctx.Context(), chatID, id); err != nil {
		h.logger.Error("failed to set state", "error", err)
	}
	h.state.SetSessionMessage(ctx.Context(), chatID, msgID)

	return h.common.edit(ctx.Context(), chatID, msgID, "🔄 <b>Выберите интервал повторения:</b>", RecurrenceEditKeyboard(id).(*telego.InlineKeyboardMarkup))
}

func (h *EditorHandlers) handleSnoozeMenu(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)
	id, idOk := callbackID(query.Data, "snooze_menu:")
	if !idOk {
		return nil
	}
	msg, msgOk := query.Message.(*telego.Message)
	if !msgOk {
		return nil
	}
	_ = id // id is embedded in the SnoozeOptionsKeyboard via query.Data parsing
	return h.common.edit(ctx.Context(), chatID, msgID, msg.Text+"\n\n💤 Отложить на:", SnoozeOptionsKeyboard(id).(*telego.InlineKeyboardMarkup))
}

func (h *EditorHandlers) handleSnoozeBack(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)
	id, idOk := callbackID(query.Data, "snooze_back:")
	if !idOk {
		return nil
	}
	msg, msgOk := query.Message.(*telego.Message)
	if !msgOk {
		return nil
	}
	// Strip the added "\n\n💤 Отложить на:"
	text := strings.Split(msg.Text, "\n\n💤")[0]
	return h.common.edit(ctx.Context(), chatID, msgID, text, NotificationKeyboard(id).(*telego.InlineKeyboardMarkup))
}

func (h *EditorHandlers) handleSnoozeApply(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)

	parts := strings.Split(query.Data, ":")
	if len(parts) != 3 {
		return nil
	}
	
	durationStr := parts[1]
	idStr := parts[2]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil
	}

	var duration time.Duration
	switch durationStr {
	case "5m":
		duration = 5 * time.Minute
	case "30m":
		duration = 30 * time.Minute
	case "1h":
		duration = time.Hour
	case "24h":
		duration = 24 * time.Hour
	default:
		return nil
	}

	if err := h.service.SnoozeReminder(ctx.Context(), chatID, id, duration); err != nil {
		return h.common.reportError(ctx.Context(), chatID, msgID, "Ошибка при откладывании", nil)
	}

	return h.common.reportSuccess(ctx.Context(), chatID, msgID, fmt.Sprintf("Отложено на %s", durationStr), nil)
}

func (h *EditorHandlers) handleTextEdit(ctx *th.Context, chatID int64, sessionID int, state, text string) error {
	id, _ := h.state.ParseEditingID(state)

	if utf8.RuneCountInString(text) > MaxReminderTextLength {
		return h.common.edit(ctx.Context(), chatID, sessionID, fmt.Sprintf("❌ <b>Слишком длинный текст</b> (макс %d)", MaxReminderTextLength), CancelEditKeyboard(id).(*telego.InlineKeyboardMarkup))
	}

	updated, err := h.service.UpdateFriendReminderText(ctx.Context(), chatID, id, text)
	if err != nil {
		return h.common.reportError(ctx.Context(), chatID, sessionID, MsgSaveError, nil)
	}

	// Notify the other party if this was a shared reminder
	if updated != nil {
		h.common.notifyOtherParty(ctx.Context(), chatID, updated, fmt.Sprintf("✏️ Друг изменил текст напоминания на: <b>%s</b>", html.EscapeString(updated.Text)))
	}

	_ = h.state.ClearState(ctx.Context(), chatID)
	return h.common.showReminderDetail(ctx.Context(), chatID, sessionID, id)
}

func (h *EditorHandlers) handleTextInterval(ctx *th.Context, chatID int64, sessionID int, state, input string) error {
	interval, err := h.parser.ParseInterval(input)

	targetID := h.state.ResolveReminderID(ctx.Context(), chatID, state, core.StateCustomIntervalPrefix)

	if err != nil {
		var cancelKb *telego.InlineKeyboardMarkup
		if targetID != 0 {
			cancelKb = CancelEditKeyboard(targetID).(*telego.InlineKeyboardMarkup)
		} else {
			cancelKb = CancelKeyboard().(*telego.InlineKeyboardMarkup)
		}
		return h.common.edit(ctx.Context(), chatID, sessionID,
			"❌ <b>Неверный формат интервала.</b>\n\nПопробуйте ещё раз, например:\n• 2 часа\n• 3 дня\n• 15 минут",
			cancelKb)
	}

	if targetID != 0 {
		// We use UpdateFriendReminderText (well, technically we only have Text and Time for friend updates right now)
		// but UpdateReminderInterval can be used too. Since it doesn't return the reminder, we fetch it.
		r, _ := h.service.GetReminder(ctx.Context(), targetID)
		if err := h.service.UpdateReminderInterval(ctx.Context(), chatID, targetID, interval); err != nil {
			return h.common.reportError(ctx.Context(), chatID, sessionID, MsgSaveError, nil)
		}
		if r != nil && r.AuthorID != 0 {
			r.Interval = interval
			h.common.notifyOtherParty(ctx.Context(), chatID, r, fmt.Sprintf("🔄 Друг изменил повтор напоминания «%s» на: <b>%s</b>", html.EscapeString(r.Text), core.FormatRecurrence(*r)))
		}
	}

	_ = h.state.ClearState(ctx.Context(), chatID)
	if targetID != 0 {
		return h.common.showReminderDetail(ctx.Context(), chatID, sessionID, targetID)
	}
	rems, _ := h.service.GetReminders(ctx.Context(), chatID)
	userLoc := h.service.GetUserLocation(ctx.Context(), chatID)
	return h.common.edit(ctx.Context(), chatID, sessionID, h.common.buildListText(ctx.Context(), chatID, rems), ListKeyboard(rems, userLoc).(*telego.InlineKeyboardMarkup))
}

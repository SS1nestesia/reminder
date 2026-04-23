package telegram

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"reminder-bot/internal/core"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

type CreatorHandlers struct {
	bot     BotAPI
	service ReminderServicer
	parser  Parserr
	state   StateManagerr
	logger  *slog.Logger
	common  *Handlers
}

func (h *CreatorHandlers) handleAddReminder(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)

	if err := h.state.SetWaitingForTextState(ctx.Context(), chatID); err != nil {
		h.logger.Error("failed to set state", "error", err)
	}
	h.state.SetSessionMessage(ctx.Context(), chatID, msgID)

	return h.common.edit(ctx.Context(), chatID, msgID, "✍️ <b>Напишите текст напоминания:</b>", CancelKeyboard().(*telego.InlineKeyboardMarkup))
}

func (h *CreatorHandlers) handleQuickTime(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, _ := h.common.callbackCtx(ctx, query)
	state, _ := h.state.GetUserState(ctx.Context(), chatID)

	var duration time.Duration
	switch strings.TrimPrefix(query.Data, "quick:") {
	case "10m":
		duration = 10 * time.Minute
	case "30m":
		duration = 30 * time.Minute
	case "1h":
		duration = time.Hour
	case "2h":
		duration = 2 * time.Hour
	default:
		return nil
	}

	userLoc := h.service.GetUserLocation(ctx.Context(), chatID)
	t := time.Now().In(userLoc).Add(duration).UTC()
	sessionID, _ := h.state.GetSessionMessage(ctx.Context(), chatID)

	// If rescheduling an existing reminder
	if id, ok := h.state.ParseRescheduleID(state); ok {
		if err := h.service.RescheduleReminder(ctx.Context(), chatID, id, t); err != nil {
			return h.common.reportError(ctx.Context(), chatID, sessionID, MsgSaveError, nil)
		}
		_ = h.state.ClearState(ctx.Context(), chatID)
		return h.common.showReminderDetail(ctx.Context(), chatID, sessionID, id)
	}

	// New reminder creation flow
	text, _ := h.state.GetPendingText(ctx.Context(), chatID)
	if text == "" {
		text = "Напоминание"
	}
	newID, err := h.service.AddReminder(ctx.Context(), chatID, text, t)
	if err != nil {
		return h.common.reportError(ctx.Context(), chatID, sessionID, MsgSaveError, nil)
	}

	return h.common.finalizeReminderCreation(ctx.Context(), chatID, sessionID, newID, t, "Напоминание сохранено!")
}

func (h *CreatorHandlers) handleRepeat(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)
	interval := strings.TrimPrefix(query.Data, "repeat:")
	if interval == "none" {
		interval = ""
	}

	state, _ := h.state.GetUserState(ctx.Context(), chatID)

	// Custom interval — ask user to type it
	if interval == "custom" {
		targetID := h.state.ResolveReminderID(ctx.Context(), chatID, state, core.StateEditRepeatPrefix)
		prefix := core.StateCustomIntervalPrefix
		if targetID != 0 {
			prefix = fmt.Sprintf("%s%d", core.StateCustomIntervalPrefix, targetID)
		}

		if err := h.state.SetState(ctx.Context(), chatID, prefix); err != nil {
			h.logger.Error("failed to set state", "error", err)
		}

		var cancelKb *telego.InlineKeyboardMarkup
		if targetID != 0 {
			cancelKb = CancelEditKeyboard(targetID).(*telego.InlineKeyboardMarkup)
		} else {
			cancelKb = CancelKeyboard().(*telego.InlineKeyboardMarkup)
		}
		return h.common.edit(ctx.Context(), chatID, msgID,
			"⚙️ <b>Введите интервал повторения.</b>\n\nПримеры:\n• 2 часа\n• 3 дня\n• 15 минут",
			cancelKb)
	}

	// Weekday picker
	if interval == "weekdays" {
		targetID := h.state.ResolveReminderID(ctx.Context(), chatID, state, core.StateEditRepeatPrefix)
		if targetID == 0 {
			h.logger.Error("cannot determine target reminder for weekdays", "state", state, "chat_id", chatID)
			return h.common.edit(ctx.Context(), chatID, msgID,
				"❌ <b>Ошибка: не удалось определить напоминание</b>", BackToMenuKeyboard().(*telego.InlineKeyboardMarkup))
		}

		if err := h.state.SetWaitingWeekdaysState(ctx.Context(), chatID, targetID); err != nil {
			h.logger.Error("failed to set state", "error", err)
		}
		return h.common.edit(ctx.Context(), chatID, msgID,
			"📅 <b>Выберите дни недели:</b>",
			WeekdaysKeyboard(0).(*telego.InlineKeyboardMarkup))
	}

	// Standard interval (daily, weekly, none) — apply it
	sessionID, _ := h.state.GetSessionMessage(ctx.Context(), chatID)
	if err := h.common.applyInterval(ctx.Context(), chatID, state, interval); err != nil {
		return h.common.reportError(ctx.Context(), chatID, sessionID, MsgSaveError, nil)
	}

	targetID := h.state.ResolveReminderID(ctx.Context(), chatID, state, core.StateEditRepeatPrefix)
	_ = h.state.ClearState(ctx.Context(), chatID)

	if targetID != 0 {
		return h.common.showReminderDetail(ctx.Context(), chatID, sessionID, targetID)
	}

	rems, _ := h.service.GetReminders(ctx.Context(), chatID)
	userLoc := h.service.GetUserLocation(ctx.Context(), chatID)
	return h.common.edit(ctx.Context(), chatID, sessionID, h.common.buildListText(ctx.Context(), chatID, rems), ListKeyboard(rems, userLoc).(*telego.InlineKeyboardMarkup))
}

func (h *CreatorHandlers) handleWeekday(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)
	data := strings.TrimPrefix(query.Data, "wd:")

	state, _ := h.state.GetUserState(ctx.Context(), chatID)
	if !strings.HasPrefix(state, core.StateWeekdaysPrefix) {
		return nil
	}

	// Reconstruct current mask from keyboard buttons
	currentMask := h.common.parseWeekdayMask(query.Message)

	if data == "done" {
		if currentMask == 0 {
			return h.bot.AnswerCallbackQuery(ctx.Context(), &telego.AnswerCallbackQueryParams{
				CallbackQueryID: query.ID,
				Text:            "Выберите хотя бы один день!",
				ShowAlert:       true,
			})
		}

		targetID := h.state.ResolveReminderID(ctx.Context(), chatID, state, core.StateWeekdaysPrefix)
		if targetID == 0 {
			h.logger.Error("failed to get target reminder for weekdays", "state", state)
			return h.common.edit(ctx.Context(), chatID, msgID, "❌ <b>Ошибка: напоминание не найдено</b>", nil)
		}

		if err := h.service.UpdateReminderWeekdays(ctx.Context(), chatID, targetID, currentMask); err != nil {
			return h.common.reportError(ctx.Context(), chatID, msgID, MsgSaveError, nil)
		}

		_ = h.state.ClearState(ctx.Context(), chatID)
		return h.common.showReminderDetail(ctx.Context(), chatID, msgID, targetID)
	}

	wdID, _ := strconv.Atoi(data)
	currentMask ^= (1 << uint(wdID))

	return h.common.edit(ctx.Context(), chatID, msgID,
		"📅 <b>Выберите дни недели:</b>",
		WeekdaysKeyboard(currentMask).(*telego.InlineKeyboardMarkup))
}

func (h *CreatorHandlers) handleCancel(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)
	_ = h.state.ClearState(ctx.Context(), chatID)
	tz, _ := h.state.GetTimezone(ctx.Context(), chatID)
	pendingCount := h.common.getPendingCount(ctx.Context(), chatID)
	return h.common.edit(ctx.Context(), chatID, msgID, MsgCancelled, MainMenuKeyboard(tz, pendingCount).(*telego.InlineKeyboardMarkup))
}

func (h *CreatorHandlers) handleTextNew(ctx *th.Context, chatID int64, sessionID int, text string) error {
	if utf8.RuneCountInString(text) > MaxReminderTextLength {
		return h.common.edit(ctx.Context(), chatID, sessionID, fmt.Sprintf("❌ <b>Слишком длинный текст</b> (макс %d)", MaxReminderTextLength), CancelKeyboard().(*telego.InlineKeyboardMarkup))
	}

	_ = h.state.SetPendingText(ctx.Context(), chatID, text)
	_ = h.state.SetWaitingForTimeState(ctx.Context(), chatID)

	return h.common.edit(ctx.Context(), chatID, sessionID,
		"✅ <b>Текст сохранён!</b>\n\nКогда напомнить?\n\nПримеры:\n• через 30 минут\n• завтра в 15:04\n• 25 марта в 14:30",
		QuickTimeKeyboard().(*telego.InlineKeyboardMarkup))
}

func (h *CreatorHandlers) handleTextTime(ctx *th.Context, chatID int64, sessionID int, state, input string) error {
	t, err := h.parser.ParseTime(input)
	if err != nil {
		var kb *telego.InlineKeyboardMarkup
		if id, ok := h.state.ParseRescheduleID(state); ok {
			kb = QuickTimeEditKeyboard(id).(*telego.InlineKeyboardMarkup)
		} else {
			kb = QuickTimeKeyboard().(*telego.InlineKeyboardMarkup)
		}
		return h.common.edit(ctx.Context(), chatID, sessionID,
			"❌ <b>Не удалось распознать время</b>\n\nПримеры: <i>через 30 минут, завтра в 15:00, 5 апреля в 19:30</i>",
			kb)
	}

	// If rescheduling an existing reminder
	if id, ok := h.state.ParseRescheduleID(state); ok {
		if err := h.service.RescheduleReminder(ctx.Context(), chatID, id, t); err != nil {
			return h.common.reportError(ctx.Context(), chatID, sessionID, MsgSaveError, nil)
		}
		_ = h.state.ClearState(ctx.Context(), chatID)
		return h.common.showReminderDetail(ctx.Context(), chatID, sessionID, id)
	}

	// New reminder creation flow
	pendingText, _ := h.state.GetPendingText(ctx.Context(), chatID)
	if pendingText == "" {
		pendingText = input
	}
	newID, err := h.service.AddReminder(ctx.Context(), chatID, pendingText, t)
	if err != nil {
		return h.common.reportError(ctx.Context(), chatID, sessionID, MsgSaveError, nil)
	}

	return h.common.finalizeReminderCreation(ctx.Context(), chatID, sessionID, newID, t, "Напоминание сохранено!")
}

package telegram

import (
	"fmt"
	"html"
	"log/slog"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

type ListHandlers struct {
	bot     BotAPI
	service ReminderServicer
	parser  Parserr
	state   StateManagerr
	logger  *slog.Logger
	common  *Handlers
}

func (h *ListHandlers) handleListReminders(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)

	rems, err := h.service.GetReminders(ctx.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get reminders", "error", err)
	}

	_ = h.state.SetSessionMessage(ctx.Context(), chatID, msgID)
	userLoc := h.service.GetUserLocation(ctx.Context(), chatID)
	return h.common.edit(ctx.Context(), chatID, msgID, h.common.buildListText(ctx.Context(), chatID, rems), ListKeyboard(rems, userLoc).(*telego.InlineKeyboardMarkup))
}

func (h *ListHandlers) handleConfirmDelete(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)

	id, ok := callbackID(query.Data, "confirm_delete:")
	if !ok {
		return nil
	}

	return h.common.edit(ctx.Context(), chatID, msgID, "🗑 <b>Вы уверены, что хотите удалить это напоминание?</b>", ConfirmDeleteKeyboard(id).(*telego.InlineKeyboardMarkup))
}

func (h *ListHandlers) handleDelete(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)

	id, ok := callbackID(query.Data, "delete:")
	if !ok {
		return nil
	}

	deleted, err := h.service.DeleteFriendReminder(ctx.Context(), chatID, id)
	if err != nil {
		return h.common.reportError(ctx.Context(), chatID, msgID, "Ошибка при удалении", nil)
	}

	if deleted != nil {
		h.common.notifyOtherParty(ctx.Context(), chatID, deleted, fmt.Sprintf("🗑 Друг удалил напоминание: <b>%s</b>", html.EscapeString(deleted.Text)))
	}

	rems, _ := h.service.GetReminders(ctx.Context(), chatID)
	userLoc := h.service.GetUserLocation(ctx.Context(), chatID)
	return h.common.edit(ctx.Context(), chatID, msgID, "✅ <b>Напоминание удалено!</b>\n\n"+h.common.buildListText(ctx.Context(), chatID, rems), ListKeyboard(rems, userLoc).(*telego.InlineKeyboardMarkup))
}

func (h *ListHandlers) handleDone(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)

	id, ok := callbackID(query.Data, "done:")
	if !ok {
		return nil
	}

	r, _ := h.service.GetReminder(ctx.Context(), id)
	if err := h.service.CompleteReminder(ctx.Context(), chatID, id); err != nil {
		return h.common.reportError(ctx.Context(), chatID, msgID, "Ошибка при выполнении", nil)
	}

	if r != nil && r.AuthorID != 0 {
		h.common.notifyOtherParty(ctx.Context(), chatID, r, fmt.Sprintf("✅ Друг отметил напоминание как выполненное: <b>%s</b>", html.EscapeString(r.Text)))
	}

	rems, _ := h.service.GetReminders(ctx.Context(), chatID)
	userLoc := h.service.GetUserLocation(ctx.Context(), chatID)
	return h.common.edit(ctx.Context(), chatID, msgID, "✅ <b>Напоминание выполнено!</b>\n\n"+h.common.buildListText(ctx.Context(), chatID, rems), ListKeyboard(rems, userLoc).(*telego.InlineKeyboardMarkup))
}

func (h *ListHandlers) handleView(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.common.callbackCtx(ctx, query)

	id, ok := callbackID(query.Data, "view:")
	if !ok {
		return nil
	}

	_ = h.state.ClearState(ctx.Context(), chatID)
	return h.common.showReminderDetail(ctx.Context(), chatID, msgID, id)
}

package telegram

import (
	"strconv"
	"strings"

	"reminder-bot/internal/core"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

func (h *Handlers) handleTextMessage(ctx *th.Context, message telego.Message) error {
	chatID := message.Chat.ID
	h.upsertUser(ctx.Context(), message.From)
	state, _ := h.state.GetUserState(ctx.Context(), chatID)

	if state == "" {
		tz, _ := h.state.GetTimezone(ctx.Context(), chatID)
		pendingCount := h.getPendingCount(ctx.Context(), chatID)
		return h.send(ctx.Context(), chatID, MsgMainMenu, MainMenuKeyboard(tz, pendingCount))
	}

	// Clean up user message
	_ = h.bot.DeleteMessage(ctx.Context(), &telego.DeleteMessageParams{ChatID: tu.ID(chatID), MessageID: message.MessageID})

	sessionID, _ := h.state.GetSessionMessage(ctx.Context(), chatID)

	switch {
	case strings.HasPrefix(state, core.StateCustomIntervalPrefix):
		return h.editor.handleTextInterval(ctx, chatID, sessionID, state, message.Text)
	case strings.HasPrefix(state, core.StateEditingPrefix):
		return h.editor.handleTextEdit(ctx, chatID, sessionID, state, message.Text)
	case state == core.StateWaitingReminderTime || strings.HasPrefix(state, core.StateReschedulePrefix):
		return h.creator.handleTextTime(ctx, chatID, sessionID, state, message.Text)
	case state == core.StateWaitingReminderText:
		return h.creator.handleTextNew(ctx, chatID, sessionID, message.Text)
	case state == core.StateWaitingTimezone:
		return h.handleTextTimezone(ctx, chatID, sessionID, message.Text)
	case strings.HasPrefix(state, "waiting_text_for:"):
		if h.friend != nil {
			friendID, ok := parseFriendTargetState(state, "waiting_text_for:")
			if ok {
				return h.friend.handleTextNewForFriend(ctx, chatID, sessionID, friendID, message.Text)
			}
		}
	case strings.HasPrefix(state, "waiting_time_for:"):
		if h.friend != nil {
			friendID, ok := parseFriendTargetState(state, "waiting_time_for:")
			if ok {
				return h.friend.handleTextTimeForFriend(ctx, chatID, sessionID, friendID, message.Text)
			}
		}
	}

	tz, _ := h.state.GetTimezone(ctx.Context(), chatID)
	pendingCount := h.getPendingCount(ctx.Context(), chatID)
	return h.send(ctx.Context(), chatID, MsgMainMenu, MainMenuKeyboard(tz, pendingCount))
}

// parseFriendTargetState extracts the friend ID from states like "waiting_text_for:12345"
func parseFriendTargetState(state, prefix string) (int64, bool) {
	if !strings.HasPrefix(state, prefix) {
		return 0, false
	}
	idStr := strings.TrimPrefix(state, prefix)
	id, err := strconv.ParseInt(idStr, 10, 64)
	return id, err == nil
}

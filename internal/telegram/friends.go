package telegram

import (
        "context"
        "fmt"
        "html"
        "log/slog"
        "strconv"
        "strings"
        "time"
        "unicode/utf8"

        "reminder-bot/internal/core"
        "reminder-bot/internal/storage"

        "github.com/mymmrac/telego"
        th "github.com/mymmrac/telego/telegohandler"
        tu "github.com/mymmrac/telego/telegoutil"
)

// FriendHandlers manages all friend-related interactions.
type FriendHandlers struct {
        bot     BotAPI
        service ReminderServicer
        friends FriendServicer
        parser  Parserr
        state   StateManagerr
        logger  *slog.Logger
        common  *Handlers
        botName string // bot username for invite links
}

// --- Friends Menu ---

func (h *FriendHandlers) handleFriendsMenu(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        friends, _ := h.friends.GetFriends(ctx.Context(), chatID)
        pending, _ := h.friends.GetPendingRequests(ctx.Context(), chatID)

        var sb strings.Builder
        sb.WriteString("👥 <b>Друзья</b>\n\n")

        if len(pending) > 0 {
                fmt.Fprintf(&sb, "🔔 <b>Входящие заявки: %d</b>\n\n", len(pending))
                for _, p := range pending {
                        name := h.common.ResolveName(ctx.Context(), p.UserID)
                        fmt.Fprintf(&sb, "• %s\n", name)
                }
                sb.WriteString("\n")
        }

        if len(friends) == 0 {
                sb.WriteString("У вас пока нет друзей.\nНажмите «Пригласить друга» чтобы отправить ссылку-приглашение!")
        } else {
                fmt.Fprintf(&sb, "Всего друзей: %d\n\n", len(friends))
                for i, f := range friends {
                        name := h.common.ResolveName(ctx.Context(), f.FriendID)
                        fmt.Fprintf(&sb, "%d. %s\n", i+1, name)
                }
        }

        return h.common.edit(ctx.Context(), chatID, msgID, sb.String(), h.friendsMenuKeyboard(ctx.Context(), friends, pending))
}

func (h *FriendHandlers) friendsMenuKeyboard(ctx context.Context, friends, pending []storage.Friend) *telego.InlineKeyboardMarkup {
        var rows [][]telego.InlineKeyboardButton

        // Pending request buttons
        for _, p := range pending {
                idStr := strconv.FormatInt(p.UserID, 10)
                name := h.common.ResolveName(ctx, p.UserID)
                rows = append(rows, tu.InlineKeyboardRow(
                        tu.InlineKeyboardButton(fmt.Sprintf("✅ Принять (%s)", name)).WithCallbackData(CBPrefixAcceptFriend+idStr),
                        tu.InlineKeyboardButton("❌").WithCallbackData(CBPrefixRejectFriend+idStr),
                ))
        }

        // Remove friend buttons
        for _, f := range friends {
                idStr := strconv.FormatInt(f.FriendID, 10)
                name := h.common.ResolveName(ctx, f.FriendID)
                rows = append(rows, tu.InlineKeyboardRow(
                        tu.InlineKeyboardButton(fmt.Sprintf("🗑 Удалить %s", name)).WithCallbackData(CBPrefixConfirmRemove+idStr),
                ))
        }

        rows = append(rows,
                tu.InlineKeyboardRow(
                        tu.InlineKeyboardButton("🔗 Пригласить друга").WithCallbackData(CBFriendsInvite),
                ),
                tu.InlineKeyboardRow(
                        tu.InlineKeyboardButton("📤 Мои напоминания друзьям").WithCallbackData(CBMyFriendReminders),
                ),
                tu.InlineKeyboardRow(
                        tu.InlineKeyboardButton("⬅️ В меню").WithCallbackData(CBBackToMenu),
                ),
        )

        return tu.InlineKeyboard(rows...)
}

// --- Invite Link ---

func (h *FriendHandlers) handleInvite(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        payload := core.GenerateInvitePayload(chatID)
        link := fmt.Sprintf("https://t.me/%s?start=%s", h.botName, payload)

        text := fmt.Sprintf("🔗 <b>Ваша ссылка-приглашение:</b>\n\n<code>%s</code>\n\nОтправьте эту ссылку своему другу!", link)
        return h.common.edit(ctx.Context(), chatID, msgID, text, BackToFriendsKeyboard())
}

// --- Accept / Reject Friend ---

func (h *FriendHandlers) handleAcceptFriend(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        fromUserID, ok := callbackID(query.Data, CBPrefixAcceptFriend)
        if !ok {
                return nil
        }

        if err := h.friends.AcceptFriendRequest(ctx.Context(), fromUserID, chatID); err != nil {
                h.logger.Error("failed to accept friend", "error", err)
                return h.common.edit(ctx.Context(), chatID, msgID, "❌ Ошибка при принятии заявки", BackToFriendsKeyboard())
        }

        // Notify the requester
        notifyText := "🎉 <b>Ваша заявка в друзья принята!</b>\n\nТеперь вы можете создавать напоминания для друга."
        _ = h.common.send(ctx.Context(), fromUserID, notifyText, nil)

        return h.common.edit(ctx.Context(), chatID, msgID, "✅ <b>Друг добавлен!</b>", BackToFriendsKeyboard())
}

func (h *FriendHandlers) handleRejectFriend(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        fromUserID, ok := callbackID(query.Data, CBPrefixRejectFriend)
        if !ok {
                return nil
        }

        if err := h.friends.RejectFriendRequest(ctx.Context(), fromUserID, chatID); err != nil {
                h.logger.Error("failed to reject friend", "error", err)
        }

        return h.common.edit(ctx.Context(), chatID, msgID, "✅ Заявка отклонена", BackToFriendsKeyboard())
}

// --- Remove Friend ---

func (h *FriendHandlers) handleConfirmRemoveFriend(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        friendID, ok := callbackID(query.Data, CBPrefixConfirmRemove)
        if !ok {
                return nil
        }

        name := h.common.ResolveName(ctx.Context(), friendID)
        text := fmt.Sprintf("🗑 <b>Удалить %s из друзей?</b>\n\nНапоминания останутся, но потеряют привязку к автору.", name)
        idStr := strconv.FormatInt(friendID, 10)
        kb := tu.InlineKeyboard(
                tu.InlineKeyboardRow(
                        tu.InlineKeyboardButton("✅ Да, удалить").WithCallbackData(CBPrefixRemoveFriend+idStr),
                        tu.InlineKeyboardButton("❌ Нет").WithCallbackData(CBFriendsMenu),
                ),
        )

        return h.common.edit(ctx.Context(), chatID, msgID, text, kb)
}

func (h *FriendHandlers) handleRemoveFriend(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        friendID, ok := callbackID(query.Data, CBPrefixRemoveFriend)
        if !ok {
                return nil
        }

        if err := h.friends.RemoveFriend(ctx.Context(), chatID, friendID); err != nil {
                h.logger.Error("failed to remove friend", "error", err)
                return h.common.edit(ctx.Context(), chatID, msgID, "❌ Ошибка при удалении", BackToFriendsKeyboard())
        }

        return h.common.edit(ctx.Context(), chatID, msgID, "✅ <b>Друг удалён</b>\n\nНапоминания сохранены, но автор очищен.", BackToFriendsKeyboard())
}

// --- Friend Reminders List ---

func (h *FriendHandlers) handleFriendRemindersList(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        rems, err := h.service.GetFriendReminders(ctx.Context(), chatID)
        if err != nil {
                h.logger.Error("failed to get friend reminders", "error", err)
        }

        _ = h.state.SetSessionMessage(ctx.Context(), chatID, msgID)
        userLoc := h.service.GetUserLocation(ctx.Context(), chatID)
        text := h.buildFriendRemindersText(ctx.Context(), rems, userLoc)
        return h.common.edit(ctx.Context(), chatID, msgID, text, FriendRemindersKeyboard(rems, userLoc))
}

func (h *FriendHandlers) buildFriendRemindersText(ctx context.Context, rems []storage.Reminder, userLoc *time.Location) string {
        if len(rems) == 0 {
                return "📬 <b>Напоминания от друзей</b>\n\nПока ничего нет."
        }

        var sb strings.Builder
        sb.WriteString("📬 <b>Напоминания от друзей</b>\n\n")
        for i, r := range rems {
                timeStr := r.NotifyAt.In(userLoc).Format("15:04 02.01")
                var authorTag string
                if r.AuthorID != 0 {
                        name := h.common.ResolveName(ctx, r.AuthorID)
                        authorTag = fmt.Sprintf("👤 От: %s", name)
                } else {
                        authorTag = "👤 Автор неизвестен"
                }
                fmt.Fprintf(&sb, "🔹 %d. <b>%s</b>\n   ⏰ %s\n   %s\n   🔄 %s\n\n",
                        i+1, html.EscapeString(r.Text), timeStr, authorTag, core.FormatRecurrence(r))
        }
        return sb.String()
}

// handleMyFriendRemindersList shows the author-side view: reminders the
// current user has created for friends.
func (h *FriendHandlers) handleMyFriendRemindersList(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        rems, err := h.service.GetOutgoingFriendReminders(ctx.Context(), chatID)
        if err != nil {
                h.logger.Error("failed to get outgoing friend reminders", "error", err)
        }

        _ = h.state.SetSessionMessage(ctx.Context(), chatID, msgID)
        userLoc := h.service.GetUserLocation(ctx.Context(), chatID)
        text := h.buildMyFriendRemindersText(ctx.Context(), rems, userLoc)
        return h.common.edit(ctx.Context(), chatID, msgID, text, MyFriendRemindersKeyboard(rems, userLoc))
}

func (h *FriendHandlers) buildMyFriendRemindersText(ctx context.Context, rems []storage.Reminder, userLoc *time.Location) string {
        if len(rems) == 0 {
                return "📤 <b>Напоминания, созданные для друзей</b>\n\nПока нет ни одного. Откройте «👥 Друзья» и нажмите «➕ Создать напоминание для друга»."
        }

        var sb strings.Builder
        sb.WriteString("📤 <b>Напоминания, созданные для друзей</b>\n\n")
        for i, r := range rems {
                timeStr := r.NotifyAt.In(userLoc).Format("15:04 02.01")
                targetName := h.common.ResolveName(ctx, r.ChatID)
                fmt.Fprintf(&sb, "🔸 %d. <b>%s</b>\n   👤 Для: %s\n   ⏰ %s\n   🔄 %s\n\n",
                        i+1, html.EscapeString(r.Text), targetName, timeStr, core.FormatRecurrence(r))
        }
        return sb.String()
}


func (h *FriendHandlers) handleFriendView(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        id, ok := callbackID(query.Data, CBPrefixFriendView)
        if !ok {
                return nil
        }

        _ = h.state.ClearState(ctx.Context(), chatID)
        return h.common.showReminderDetail(ctx.Context(), chatID, msgID, id)
}

func (h *FriendHandlers) handleFriendConfirmDelete(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        id, ok := callbackID(query.Data, CBPrefixFriendConfDel)
        if !ok {
                return nil
        }

        idStr := strconv.FormatInt(id, 10)
        return h.common.edit(ctx.Context(), chatID, msgID, "🗑 <b>Удалить это напоминание от друга?</b>",
                tu.InlineKeyboard(
                        tu.InlineKeyboardRow(
                                tu.InlineKeyboardButton("✅ Да, удалить").WithCallbackData(CBPrefixFriendDelete+idStr),
                                tu.InlineKeyboardButton("❌ Нет").WithCallbackData(CBFriendReminders),
                        ),
                ))
}

func (h *FriendHandlers) handleFriendDelete(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        id, ok := callbackID(query.Data, CBPrefixFriendDelete)
        if !ok {
                return nil
        }

        deleted, err := h.service.DeleteFriendReminder(ctx.Context(), chatID, id)
        if err != nil {
                return h.common.reportError(ctx.Context(), chatID, msgID, "Ошибка при удалении", nil)
        }

        // Notify the other party
        if deleted != nil {
                h.common.notifyOtherParty(ctx.Context(), chatID, deleted, fmt.Sprintf("🗑 Друг удалил напоминание: <b>%s</b>", html.EscapeString(deleted.Text)))
        }

        rems, _ := h.service.GetFriendReminders(ctx.Context(), chatID)
        userLoc := h.service.GetUserLocation(ctx.Context(), chatID)
        text := "✅ <b>Напоминание удалено!</b>\n\n" + h.buildFriendRemindersText(ctx.Context(), rems, userLoc)
        return h.common.edit(ctx.Context(), chatID, msgID, text, FriendRemindersKeyboard(rems, userLoc))
}

// --- Create Reminder For Friend (target selection) ---

func (h *FriendHandlers) handleCreateForChoice(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        friends, _ := h.friends.GetFriends(ctx.Context(), chatID)
        if len(friends) == 0 {
                // No friends — go straight to self
                if err := h.state.SetWaitingForTextState(ctx.Context(), chatID); err != nil {
                        h.logger.Error("failed to set state", "error", err)
                }
                _ = h.state.SetSessionMessage(ctx.Context(), chatID, msgID)
                return h.common.edit(ctx.Context(), chatID, msgID, "✍️ <b>Напишите текст напоминания:</b>", CancelKeyboard().(*telego.InlineKeyboardMarkup))
        }

        // Show target selection keyboard
        var rows [][]telego.InlineKeyboardButton
        rows = append(rows, tu.InlineKeyboardRow(
                tu.InlineKeyboardButton("👤 Для себя").WithCallbackData(CBCreateForSelf),
        ))
        for _, f := range friends {
                idStr := strconv.FormatInt(f.FriendID, 10)
                name := h.common.ResolveName(ctx.Context(), f.FriendID)
                rows = append(rows, tu.InlineKeyboardRow(
                        tu.InlineKeyboardButton(fmt.Sprintf("👤 %s", name)).WithCallbackData(CBPrefixCreateFor+idStr),
                ))
        }
        rows = append(rows, tu.InlineKeyboardRow(
                tu.InlineKeyboardButton("❌ Отменить").WithCallbackData(CBCancel),
        ))

        return h.common.edit(ctx.Context(), chatID, msgID, "📝 <b>Для кого создать напоминание?</b>", tu.InlineKeyboard(rows...))
}

func (h *FriendHandlers) handleCreateForTarget(ctx *th.Context, query telego.CallbackQuery) error {
        chatID, msgID := h.common.callbackCtx(ctx, query)

        target := strings.TrimPrefix(query.Data, CBPrefixCreateFor)

        if target == "self" {
                // Normal flow - clear any friend target from state
                if err := h.state.SetWaitingForTextState(ctx.Context(), chatID); err != nil {
                        h.logger.Error("failed to set state", "error", err)
                }
                _ = h.state.SetSessionMessage(ctx.Context(), chatID, msgID)
                return h.common.edit(ctx.Context(), chatID, msgID, "✍️ <b>Напишите текст напоминания:</b>", CancelKeyboard().(*telego.InlineKeyboardMarkup))
        }

        friendID, err := strconv.ParseInt(target, 10, 64)
        if err != nil {
                return nil
        }

        // Set state to friend creation mode: "waiting_text_for:<friendID>"
        state := fmt.Sprintf("waiting_text_for:%d", friendID)
        if err := h.state.SetState(ctx.Context(), chatID, state); err != nil {
                h.logger.Error("failed to set state", "error", err)
        }
        _ = h.state.SetSessionMessage(ctx.Context(), chatID, msgID)

        name := h.common.ResolveName(ctx.Context(), friendID)
        return h.common.edit(ctx.Context(), chatID, msgID,
                fmt.Sprintf("✍️ <b>Напишите текст напоминания для %s:</b>", name),
                CancelKeyboard().(*telego.InlineKeyboardMarkup))
}

// handleTextNewForFriend handles text input when creating a reminder for a friend.
func (h *FriendHandlers) handleTextNewForFriend(ctx *th.Context, chatID int64, sessionID int, friendID int64, text string) error {
        if utf8.RuneCountInString(text) > MaxReminderTextLength {
                return h.common.edit(ctx.Context(), chatID, sessionID,
                        fmt.Sprintf("❌ <b>Слишком длинный текст</b> (макс %d)", MaxReminderTextLength),
                        CancelKeyboard().(*telego.InlineKeyboardMarkup))
        }

        _ = h.state.SetPendingText(ctx.Context(), chatID, text)

        // Set state to "waiting_time_for:<friendID>"
        state := fmt.Sprintf("waiting_time_for:%d", friendID)
        _ = h.state.SetState(ctx.Context(), chatID, state)

        return h.common.edit(ctx.Context(), chatID, sessionID,
                "✅ <b>Текст сохранён!</b>\n\n"+MsgAskTime,
                QuickTimeKeyboard().(*telego.InlineKeyboardMarkup))
}

// handleTextTimeForFriend handles time input when creating a reminder for a friend.
func (h *FriendHandlers) handleTextTimeForFriend(ctx *th.Context, chatID int64, sessionID int, friendID int64, input string) error {
        t, err := h.parser.ParseTime(input)
        if err != nil {
                return h.common.edit(ctx.Context(), chatID, sessionID,
                        MsgTimeParseError,
                        QuickTimeKeyboard().(*telego.InlineKeyboardMarkup))
        }

        pendingText, _ := h.state.GetPendingText(ctx.Context(), chatID)
        if pendingText == "" {
                pendingText = input
        }

        newID, err := h.service.AddReminderForFriend(ctx.Context(), chatID, friendID, pendingText, t)
        if err != nil {
                return h.common.reportError(ctx.Context(), chatID, sessionID, MsgSaveError, nil)
        }

        // Notify the friend immediately.
        userLoc := h.service.GetUserLocation(ctx.Context(), chatID)
        authorName := h.common.ResolveName(ctx.Context(), chatID)
        friendNotification := fmt.Sprintf("🔔 <b>Новое напоминание от %s!</b>\n\n📝 %s\n⏰ %s",
                authorName, html.EscapeString(pendingText), t.In(userLoc).Format("02.01.2006 15:04"))
        _ = h.common.send(ctx.Context(), friendID, friendNotification, nil)

        // Re-use the shared "now configure recurrence" flow — the reminder ID is
        // stored in pending_reminder_id and applyInterval/Weekdays pull it from
        // there (both work on friend reminders after the author/owner access
        // widening in ReminderService.withReminder).
        friendName := h.common.ResolveName(ctx.Context(), friendID)
        confirmation := fmt.Sprintf("✅ <b>Напоминание для %s сохранено!</b>", friendName)
        return h.common.finalizeReminderCreation(ctx.Context(), chatID, sessionID, newID, t, confirmation)
}

// --- Helper: notify other party ---

// notifyOtherParty is now a method of the common Handlers struct in handlers.go

// --- Keyboards ---

func PendingFriendKeyboard(userID int64) *telego.InlineKeyboardMarkup {
        idStr := strconv.FormatInt(userID, 10)
        return tu.InlineKeyboard(
                tu.InlineKeyboardRow(
                        tu.InlineKeyboardButton("✅ Принять").WithCallbackData(CBPrefixAcceptFriend+idStr),
                        tu.InlineKeyboardButton("❌ Отклонить").WithCallbackData(CBPrefixRejectFriend+idStr),
                ),
        )
}

func BackToFriendsKeyboard() *telego.InlineKeyboardMarkup {
        return tu.InlineKeyboard(
                tu.InlineKeyboardRow(
                        tu.InlineKeyboardButton("⬅️ К друзьям").WithCallbackData(CBFriendsMenu),
                        tu.InlineKeyboardButton("⬅️ В меню").WithCallbackData(CBBackToMenu),
                ),
        )
}

func FriendRemindersKeyboard(reminders []storage.Reminder, userLoc *time.Location) *telego.InlineKeyboardMarkup {
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
                        tu.InlineKeyboardButton(btnText).WithCallbackData(CBPrefixFriendView+idStr),
                ))
        }

        // Both branches produced identical "back to menu" rows; simplify.
        rows = append(rows, tu.InlineKeyboardRow(
                tu.InlineKeyboardButton("⬅️ В меню").WithCallbackData(CBBackToMenu),
        ))

        return tu.InlineKeyboard(rows...)
}


// MyFriendRemindersKeyboard renders the list of author-side (outgoing)
// friend reminders. Each row links to the shared view/edit detail — the
// author has full access because the service widens ownership checks to
// author-OR-owner for friend reminders.
func MyFriendRemindersKeyboard(reminders []storage.Reminder, userLoc *time.Location) *telego.InlineKeyboardMarkup {
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
                        tu.InlineKeyboardButton(btnText).WithCallbackData(CBPrefixFriendView+idStr),
                ))
        }

        rows = append(rows, tu.InlineKeyboardRow(
                tu.InlineKeyboardButton("⬅️ Назад").WithCallbackData(CBFriendsMenu),
        ))

        return tu.InlineKeyboard(rows...)
}

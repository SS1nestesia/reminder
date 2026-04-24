package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"reminder-bot/internal/core"
	"reminder-bot/internal/storage"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

type Handlers struct {
	bot     BotAPI
	service ReminderServicer
	parser  Parserr
	state   StateManagerr
	logger  *slog.Logger

	creator *CreatorHandlers
	editor  *EditorHandlers
	list    *ListHandlers
	friend  *FriendHandlers
}

func NewHandlers(b BotAPI, service ReminderServicer, friends FriendServicer, parser Parserr, state StateManagerr, botName string, logger *slog.Logger) *Handlers {
	h := &Handlers{
		bot:     b,
		service: service,
		parser:  parser,
		state:   state,
		logger:  logger,
	}

	h.creator = &CreatorHandlers{bot: b, service: service, parser: parser, state: state, logger: logger, common: h}
	h.editor = &EditorHandlers{bot: b, service: service, parser: parser, state: state, logger: logger, common: h}
	h.list = &ListHandlers{bot: b, service: service, parser: parser, state: state, logger: logger, common: h}
	if friends != nil {
		h.friend = &FriendHandlers{bot: b, service: service, friends: friends, parser: parser, state: state, logger: logger, common: h, botName: botName}
	}

	return h
}

// callbackID parses a reminder ID from callback data after the given prefix.
func callbackID(data, prefix string) (int64, bool) {
	if !strings.HasPrefix(data, prefix) {
		return 0, false
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(data, prefix), 10, 64)
	return id, err == nil
}

// callbackCtx extracts common callback query context and answers the query.
func (h *Handlers) callbackCtx(ctx *th.Context, query telego.CallbackQuery) (chatID int64, msgID int) {
	h.answer(ctx.Context(), query.ID)
	return query.Message.GetChat().ID, query.Message.GetMessageID()
}

// finalizeReminderCreation is the shared post-creation logic for quick time and text time flows.
// It stores the pending reminder, shows the recurrence keyboard, and sets the waiting_recurrence state.
func (h *Handlers) finalizeReminderCreation(ctx context.Context, chatID int64, sessionID int, id int64, notifyAt time.Time, confirmation string) error {
	if err := h.state.SetPendingReminder(ctx, chatID, id); err != nil {
		h.logger.Error("failed to set pending reminder", "id", id, "error", err)
	}

	userLoc := h.service.GetUserLocation(ctx, chatID)
	msgText := fmt.Sprintf("%s\n⏰ %s\n\n🔄 <b>Повторять это напоминание?</b>", confirmation, notifyAt.In(userLoc).Format("02.01.2006 15:04"))
	if err := h.edit(ctx, chatID, sessionID, msgText, RecurrenceKeyboard().(*telego.InlineKeyboardMarkup)); err != nil {
		h.logger.Error("failed to edit message", "error", err)
	}

	_ = h.state.SetWaitingRecurrenceState(ctx, chatID)
	return nil
}

// --- Handler Registration ---

func (h *Handlers) RegisterAll(bh *th.BotHandler) {
	// Commands
	bh.HandleMessage(h.handleStart, th.CommandEqual("start"))

	// Callback Queries
	if h.friend != nil {
		// When friends feature is enabled, intercept "add_reminder" to show target selection
		bh.HandleCallbackQuery(h.friend.handleCreateForChoice, th.CallbackDataEqual(CBAddReminder))
	} else {
		bh.HandleCallbackQuery(h.creator.handleAddReminder, th.CallbackDataEqual(CBAddReminder))
	}
	bh.HandleCallbackQuery(h.list.handleListReminders, th.CallbackDataEqual(CBListReminders))
	bh.HandleCallbackQuery(h.handleBackToMenu, th.CallbackDataEqual(CBBackToMenu))
	bh.HandleCallbackQuery(h.list.handleConfirmDelete, th.CallbackDataPrefix(CBPrefixConfirmDelete))
	bh.HandleCallbackQuery(h.list.handleDelete, th.CallbackDataPrefix(CBPrefixDelete))
	bh.HandleCallbackQuery(h.editor.handleEditText, th.CallbackDataPrefix(CBPrefixEditText))
	bh.HandleCallbackQuery(h.editor.handleEditTime, th.CallbackDataPrefix(CBPrefixEditTime))
	bh.HandleCallbackQuery(h.editor.handleEditRepeat, th.CallbackDataPrefix(CBPrefixEditRepeat))
	bh.HandleCallbackQuery(h.list.handleView, th.CallbackDataPrefix(CBPrefixView))
	bh.HandleCallbackQuery(h.list.handleDone, th.CallbackDataPrefix(CBPrefixDone))
	bh.HandleCallbackQuery(h.editor.handleReschedule, th.CallbackDataPrefix(CBPrefixReschedule))
	bh.HandleCallbackQuery(h.editor.handleSnoozeMenu, th.CallbackDataPrefix(CBPrefixSnoozeMenu))
	bh.HandleCallbackQuery(h.editor.handleSnoozeApply, th.CallbackDataPrefix(CBPrefixSnooze))
	bh.HandleCallbackQuery(h.editor.handleSnoozeBack, th.CallbackDataPrefix(CBPrefixSnoozeBack))
	bh.HandleCallbackQuery(h.creator.handleQuickTime, th.CallbackDataPrefix(CBPrefixQuick))
	bh.HandleCallbackQuery(h.creator.handleRepeat, th.CallbackDataPrefix(CBPrefixRepeat))
	bh.HandleCallbackQuery(h.creator.handleWeekday, th.CallbackDataPrefix(CBPrefixWeekday))
	bh.HandleCallbackQuery(h.creator.handleCancel, th.CallbackDataEqual(CBCancel))
	bh.HandleCallbackQuery(h.handleSetupTimezone, th.CallbackDataEqual(CBSetupTimezone))
	bh.HandleCallbackQuery(h.handleTimezoneChoice, th.CallbackDataPrefix(CBPrefixTimezone))

	// Friends callbacks
	if h.friend != nil {
		bh.HandleCallbackQuery(h.friend.handleFriendsMenu, th.CallbackDataEqual(CBFriendsMenu))
		bh.HandleCallbackQuery(h.friend.handleInvite, th.CallbackDataEqual(CBFriendsInvite))
		bh.HandleCallbackQuery(h.friend.handleFriendRemindersList, th.CallbackDataEqual(CBFriendReminders))
		bh.HandleCallbackQuery(h.friend.handleAcceptFriend, th.CallbackDataPrefix(CBPrefixAcceptFriend))
		bh.HandleCallbackQuery(h.friend.handleRejectFriend, th.CallbackDataPrefix(CBPrefixRejectFriend))
		bh.HandleCallbackQuery(h.friend.handleConfirmRemoveFriend, th.CallbackDataPrefix(CBPrefixConfirmRemove))
		bh.HandleCallbackQuery(h.friend.handleRemoveFriend, th.CallbackDataPrefix(CBPrefixRemoveFriend))
		bh.HandleCallbackQuery(h.friend.handleCreateForTarget, th.CallbackDataPrefix(CBPrefixCreateFor))
		bh.HandleCallbackQuery(h.friend.handleFriendView, th.CallbackDataPrefix(CBPrefixFriendView))
		bh.HandleCallbackQuery(h.friend.handleFriendConfirmDelete, th.CallbackDataPrefix(CBPrefixFriendConfDel))
		bh.HandleCallbackQuery(h.friend.handleFriendDelete, th.CallbackDataPrefix(CBPrefixFriendDelete))
	}

	// Text Messages
	bh.HandleMessage(h.handleTextMessage,
		th.AnyMessageWithText(),
		th.Not(th.AnyCommand()),
		func(ctx context.Context, update telego.Update) bool {
			return update.Message == nil || update.Message.Location == nil
		},
	)
}

// --- Handlers ---

func (h *Handlers) handleStart(ctx *th.Context, message telego.Message) error {
	chatID := message.Chat.ID
	h.upsertUser(ctx.Context(), message.From)
	pendingCount := h.getPendingCount(ctx.Context(), chatID)

	// Check for invite deep link: /start invite_<chatID>
	args := strings.TrimSpace(strings.TrimPrefix(message.Text, "/start"))
	if inviterID, ok := core.ParseInvitePayload(args); ok && h.friend != nil {
		// Verify the inviter actually exists
		if _, err := h.friend.friends.GetUserInfo(ctx.Context(), inviterID); err != nil {
			tz, _ := h.state.GetTimezone(ctx.Context(), chatID)
			return h.send(ctx.Context(), chatID, "❌ Пользователь не найден.", MainMenuKeyboard(tz, pendingCount))
		}

		// Don't allow self-invite
		if inviterID == chatID {
			tz, _ := h.state.GetTimezone(ctx.Context(), chatID)
			return h.send(ctx.Context(), chatID, "❌ Вы не можете добавить себя в друзья!", MainMenuKeyboard(tz, pendingCount))
		}

		// Check if already friends
		isFriend, _ := h.friend.friends.IsFriend(ctx.Context(), chatID, inviterID)
		if isFriend {
			tz, _ := h.state.GetTimezone(ctx.Context(), chatID)
			inviterName := h.ResolveName(ctx.Context(), inviterID)
			return h.send(ctx.Context(), chatID, fmt.Sprintf("👥 Вы уже друзья с <b>%s</b>!", inviterName), MainMenuKeyboard(tz, pendingCount))
		}

		// Check if pending already
		hasPending, _ := h.friend.friends.HasPendingRequest(ctx.Context(), chatID, inviterID)
		if hasPending {
			tz, _ := h.state.GetTimezone(ctx.Context(), chatID)
			inviterName := h.ResolveName(ctx.Context(), inviterID)
			return h.send(ctx.Context(), chatID, fmt.Sprintf("⏳ Заявка для <b>%s</b> уже отправлена, ожидает подтверждения.", inviterName), MainMenuKeyboard(tz, pendingCount))
		}

		// Send friend request
		if err := h.friend.friends.SendFriendRequest(ctx.Context(), chatID, inviterID); err != nil {
			h.logger.Error("failed to send friend request", "error", err)
			tz, _ := h.state.GetTimezone(ctx.Context(), chatID)
			return h.send(ctx.Context(), chatID, "❌ Ошибка при создании заявки.", MainMenuKeyboard(tz, pendingCount))
		}

		// Resolve both sides' names for friendly messaging.
		inviteeName := h.ResolveName(ctx.Context(), chatID)
		inviterName := h.ResolveName(ctx.Context(), inviterID)

		// Notify the inviter that they have a new request, WITH a friendly name.
		_ = h.send(ctx.Context(), inviterID,
			fmt.Sprintf("🔔 <b>Новая заявка в друзья!</b>\n\n<b>%s</b> хочет стать вашим другом.", inviteeName),
			PendingFriendKeyboard(chatID))

		tz, _ := h.state.GetTimezone(ctx.Context(), chatID)
		return h.send(ctx.Context(), chatID,
			fmt.Sprintf("✅ <b>Заявка в друзья отправлена!</b>\n\nОжидаем подтверждения от <b>%s</b>.", inviterName),
			MainMenuKeyboard(tz, pendingCount))
	}

	tz, _ := h.state.GetTimezone(ctx.Context(), chatID)
	return h.send(ctx.Context(), chatID, MsgMainMenu, MainMenuKeyboard(tz, pendingCount))
}

func (h *Handlers) handleBackToMenu(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.callbackCtx(ctx, query)
	_ = h.state.ClearState(ctx.Context(), chatID)
	tz, _ := h.state.GetTimezone(ctx.Context(), chatID)
	pendingCount := h.getPendingCount(ctx.Context(), chatID)
	return h.edit(ctx.Context(), chatID, msgID, MsgMainMenu, MainMenuKeyboard(tz, pendingCount).(*telego.InlineKeyboardMarkup))
}

func (h *Handlers) handleSetupTimezone(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.callbackCtx(ctx, query)

	if err := h.state.SetWaitingTimezoneState(ctx.Context(), chatID); err != nil {
		h.logger.Error("failed to set state", "error", err)
	}

	msg := "Укажите ваш часовой пояс.\n\n👇 Выберите из списка или отправьте текстом название вашего города (например, «Москва»), либо точный пояс/смещение (например, «UTC+3» или «Europe/Moscow»)"
	return h.edit(ctx.Context(), chatID, msgID, msg, TimezoneQuickKeyboard().(*telego.InlineKeyboardMarkup))
}

func (h *Handlers) handleTimezoneChoice(ctx *th.Context, query telego.CallbackQuery) error {
	chatID, msgID := h.callbackCtx(ctx, query)

	tzName := strings.TrimPrefix(query.Data, "tz:")
	if tzName == "" {
		return nil
	}

	oldTzName, _ := h.state.GetTimezone(ctx.Context(), chatID)

	if err := h.state.SetTimezone(ctx.Context(), chatID, tzName); err != nil {
		h.logger.Error("failed to set timezone", "error", err)
		return h.edit(ctx.Context(), chatID, msgID, "❌ Ошибка при сохранении часового пояса.", MainMenuKeyboard("", 0).(*telego.InlineKeyboardMarkup))
	}

	if oldTzName != "" && oldTzName != tzName {
		_ = h.service.UpdateTimezoneForReminders(ctx.Context(), chatID, oldTzName)
	}

	_ = h.state.ClearState(ctx.Context(), chatID)
	pendingCount := h.getPendingCount(ctx.Context(), chatID)
	msg := fmt.Sprintf("✅ Ваш часовой пояс установлен: <b>%s</b>", core.FormatTimezone(tzName))
	return h.edit(ctx.Context(), chatID, msgID, msg, MainMenuKeyboard(tzName, pendingCount).(*telego.InlineKeyboardMarkup))
}

func (h *Handlers) handleTextTimezone(ctx *th.Context, chatID int64, sessionID int, text string) error {
	tzName := core.ParseTimezoneAlias(text)

	// If alias not found, try parsing as exact IANA timezone
	if tzName == "" {
		if _, err := time.LoadLocation(strings.TrimSpace(text)); err == nil {
			tzName = strings.TrimSpace(text)
		}
	}

	// Try geocoding as a city name if still empty
	if tzName == "" {
		lat, lon, err := core.GeocodeCity(ctx.Context(), text)
		if err != nil {
			h.logger.Warn("failed to geocode city", "city", text, "error", err)
			return h.edit(ctx.Context(), chatID, sessionID, "❌ <b>Город не найден</b>\n\nПопробуйте написать название по-другому (например, «Москва» или «Лондон»).", CancelKeyboard().(*telego.InlineKeyboardMarkup))
		}

		tzName = core.GetTimezoneName(lat, lon)
		if tzName == "" {
			tzName = "Europe/Moscow"
		}
	}

	oldTzName, _ := h.state.GetTimezone(ctx.Context(), chatID)

	if err := h.state.SetTimezone(ctx.Context(), chatID, tzName); err != nil {
		h.logger.Error("failed to set timezone", "error", err)
		return h.edit(ctx.Context(), chatID, sessionID, "❌ Ошибка при сохранении часового пояса.", MainMenuKeyboard("", 0).(*telego.InlineKeyboardMarkup))
	}

	if oldTzName != "" && oldTzName != tzName {
		_ = h.service.UpdateTimezoneForReminders(ctx.Context(), chatID, oldTzName)
	}

	_ = h.state.ClearState(ctx.Context(), chatID)
	pendingCount := h.getPendingCount(ctx.Context(), chatID)
	msg := fmt.Sprintf("✅ Ваш часовой пояс установлен: <b>%s</b>", core.FormatTimezone(tzName))
	return h.edit(ctx.Context(), chatID, sessionID, msg, MainMenuKeyboard(tzName, pendingCount).(*telego.InlineKeyboardMarkup))
}

// --- Helpers used by sub-handlers ---

// applyInterval updates the reminder's interval based on the current state.
func (h *Handlers) applyInterval(ctx context.Context, chatID int64, state, interval string) error {
	targetID := h.state.ResolveReminderID(ctx, chatID, state, core.StateEditRepeatPrefix)
	if targetID != 0 {
		return h.service.UpdateReminderInterval(ctx, chatID, targetID, interval)
	}
	return nil
}

// parseWeekdayMask reconstructs the weekday bitmask from the current keyboard state.
func (h *Handlers) parseWeekdayMask(msg telego.MaybeInaccessibleMessage) int {
	m, ok := msg.(*telego.Message)
	if !ok || m == nil {
		return 0
	}
	markup := m.ReplyMarkup
	if markup == nil {
		return 0
	}
	mask := 0
	for _, row := range markup.InlineKeyboard {
		for _, btn := range row {
			if strings.HasPrefix(btn.Text, "✅") && strings.HasPrefix(btn.CallbackData, "wd:") {
				wdID, err := strconv.Atoi(strings.TrimPrefix(btn.CallbackData, "wd:"))
				if err != nil || wdID < 0 || wdID > 6 {
					continue // skip "wd:done" and any out-of-range values
				}
				mask |= 1 << uint(wdID)
			}
		}
	}
	return mask
}

// upsertUser saves Telegram user info to our database for better UI (names instead of IDs).
func (h *Handlers) upsertUser(ctx context.Context, from *telego.User) {
	if from == nil || h.friend == nil {
		return
	}
	user := &storage.User{
		ChatID:    from.ID,
		FirstName: from.FirstName,
		LastName:  from.LastName,
		Username:  from.Username,
	}
	if err := h.friend.friends.UpsertUser(ctx, user); err != nil {
		h.logger.Warn("failed to upsert user info", "id", from.ID, "error", err)
	}
}

// ResolveName returns a user-friendly name for a chat ID.
// It first checks the local database cache. If missing, it tries to fetch from Telegram.
func (h *Handlers) ResolveName(ctx context.Context, chatID int64) string {
	if h.friend == nil {
		return fmt.Sprintf("Пользователь %d", chatID)
	}

	u, err := h.friend.friends.GetUserInfo(ctx, chatID)
	if err == nil && u != nil {
		return formatUserName(u)
	}

	// Cache miss: try fetching from Telegram
	chat, err := h.bot.GetChat(ctx, &telego.GetChatParams{ChatID: tu.ID(chatID)})
	if err == nil && chat != nil {
		// Found! Cache it for later
		userToCache := &storage.User{
			ChatID:    chat.ID,
			FirstName: chat.FirstName,
			LastName:  chat.LastName,
			Username:  chat.Username,
		}
		_ = h.friend.friends.UpsertUser(ctx, userToCache)
		return formatUserName(&storage.User{
			FirstName: chat.FirstName,
			LastName:  chat.LastName,
			Username:  chat.Username,
		})
	}

	return fmt.Sprintf("Пользователь %d", chatID)
}

func formatUserName(u *storage.User) string {
	if u.Username != "" {
		return "@" + u.Username
	}
	if u.LastName != "" {
		return u.FirstName + " " + u.LastName
	}
	return u.FirstName
}

// notifyOtherParty sends a message to the "other" person involved in a shared reminder.
// If actorChatID is the owner, it notifies the author. If it's the author, it notifies the owner.
func (h *Handlers) notifyOtherParty(ctx context.Context, actorChatID int64, r *storage.Reminder, message string) {
	if r == nil || r.AuthorID == 0 {
		return
	}
	// Figure out who should be notified
	var targetID int64
	if actorChatID == r.ChatID {
		targetID = r.AuthorID
	} else {
		targetID = r.ChatID
	}
	if targetID == 0 || targetID == actorChatID {
		return
	}
	name := h.ResolveName(ctx, actorChatID)
	_ = h.send(ctx, targetID, fmt.Sprintf("👤 <b>%s</b> %s", name, message), nil)
}

func (h *Handlers) getPendingCount(ctx context.Context, chatID int64) int {
	if h.friend == nil || h.friend.friends == nil {
		return 0
	}
	pending, _ := h.friend.friends.GetPendingRequests(ctx, chatID)
	return len(pending)
}

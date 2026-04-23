package telegram

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mymmrac/telego"
	"reminder-bot/internal/storage"
)

// --- Mock Service ---

type mockService struct {
	mu        sync.Mutex
	reminders map[int64][]storage.Reminder
	nextID    int64
	userLoc   *time.Location

	// Error injection
	addErr      error
	getErr      error
	deleteErr   error
	completeErr error
	reschedErr  error
	snoozeErr   error
	updateTxtErr error
	updateIntErr error
	updateWdErr  error
}

func newMockService() *mockService {
	return &mockService{
		reminders: make(map[int64][]storage.Reminder),
		nextID:    1,
		userLoc:   time.UTC,
	}
}

func (m *mockService) AddReminder(_ context.Context, chatID int64, text string, notifyAt time.Time) (int64, error) {
	if m.addErr != nil {
		return 0, m.addErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextID
	m.nextID++
	m.reminders[chatID] = append(m.reminders[chatID], storage.Reminder{
		ID: id, ChatID: chatID, Text: text, NotifyAt: notifyAt,
	})
	return id, nil
}

func (m *mockService) GetReminder(_ context.Context, id int64) (*storage.Reminder, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rems := range m.reminders {
		for i := range rems {
			if rems[i].ID == id {
				return &rems[i], nil
			}
		}
	}
	return nil, storage.ErrNotFound
}

func (m *mockService) GetReminders(_ context.Context, chatID int64) ([]storage.Reminder, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reminders[chatID], nil
}

func (m *mockService) DeleteReminder(_ context.Context, chatID int64, id int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rems := m.reminders[chatID]
	for i := range rems {
		if rems[i].ID == id {
			m.reminders[chatID] = append(rems[:i], rems[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockService) CompleteReminder(_ context.Context, _ int64, _ int64) error {
	return m.completeErr
}

func (m *mockService) RescheduleReminder(_ context.Context, _ int64, _ int64, _ time.Time) error {
	return m.reschedErr
}

func (m *mockService) SnoozeReminder(_ context.Context, _ int64, _ int64, _ time.Duration) error {
	return m.snoozeErr
}

func (m *mockService) UpdateReminderText(_ context.Context, chatID int64, id int64, text string) error {
	if m.updateTxtErr != nil {
		return m.updateTxtErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rems := m.reminders[chatID]
	for i := range rems {
		if rems[i].ID == id {
			rems[i].Text = text
			return nil
		}
	}
	return nil
}

func (m *mockService) UpdateReminderInterval(_ context.Context, _ int64, _ int64, _ string) error {
	return m.updateIntErr
}

func (m *mockService) UpdateReminderWeekdays(_ context.Context, _ int64, _ int64, _ int) error {
	return m.updateWdErr
}

func (m *mockService) GetUserLocation(_ context.Context, _ int64) *time.Location {
	if m.userLoc != nil {
		return m.userLoc
	}
	return time.UTC
}

// --- Friend Reminder methods for Mock Service ---
func (m *mockService) AddReminderForFriend(_ context.Context, _, chatID int64, text string, notifyAt time.Time) (int64, error) {
	if m.addErr != nil {
		return 0, m.addErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextID
	m.nextID++
	m.reminders[chatID] = append(m.reminders[chatID], storage.Reminder{
		ID: id, ChatID: chatID, Text: text, NotifyAt: notifyAt, AuthorID: 999,
	})
	return id, nil
}

func (m *mockService) GetFriendReminders(_ context.Context, chatID int64) ([]storage.Reminder, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var friendRems []storage.Reminder
	for _, r := range m.reminders[chatID] {
		if r.AuthorID != 0 {
			friendRems = append(friendRems, r)
		}
	}
	return friendRems, nil
}

func (m *mockService) DeleteFriendReminder(_ context.Context, _ int64, _ int64) (*storage.Reminder, error) {
	return nil, m.deleteErr
}

func (m *mockService) UpdateFriendReminderText(_ context.Context, _ int64, id int64, text string) (*storage.Reminder, error) {
	return nil, m.updateTxtErr
}

func (m *mockService) UpdateFriendReminderTime(_ context.Context, _ int64, _ int64, _ time.Time) (*storage.Reminder, error) {
	return nil, m.reschedErr
}

// --- Mock Friend Service ---
type mockFriendService struct{}

func (m *mockFriendService) SendFriendRequest(ctx context.Context, userID, friendID int64) error { return nil }
func (m *mockFriendService) AcceptFriendRequest(ctx context.Context, fromUserID, toUserID int64) error { return nil }
func (m *mockFriendService) RejectFriendRequest(ctx context.Context, fromUserID, toUserID int64) error { return nil }
func (m *mockFriendService) RemoveFriend(ctx context.Context, userID, friendID int64) error { return nil }
func (m *mockFriendService) GetFriends(ctx context.Context, userID int64) ([]storage.Friend, error) { return nil, nil }
func (m *mockFriendService) GetPendingRequests(ctx context.Context, userID int64) ([]storage.Friend, error) { return nil, nil }
func (m *mockFriendService) IsFriend(ctx context.Context, userID, friendID int64) (bool, error) { return true, nil }
func (m *mockFriendService) HasPendingRequest(ctx context.Context, userID, friendID int64) (bool, error) { return false, nil }
func (m *mockFriendService) UpsertUser(ctx context.Context, user *storage.User) error { return nil }
func (m *mockFriendService) GetUserInfo(ctx context.Context, chatID int64) (*storage.User, error) { return nil, nil }

// --- Mock State ---

type mockState struct {
	mu            sync.Mutex
	states        map[int64]string
	pendingText   map[int64]string
	pendingID     map[int64]int64
	sessionMsgs   map[int64]int
	timezones     map[int64]string

	setStateErr   error
	clearErr      error
}

func newMockState() *mockState {
	return &mockState{
		states:      make(map[int64]string),
		pendingText: make(map[int64]string),
		pendingID:   make(map[int64]int64),
		sessionMsgs: make(map[int64]int),
		timezones:   make(map[int64]string),
	}
}

func (m *mockState) GetUserState(_ context.Context, chatID int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states[chatID], nil
}

func (m *mockState) SetState(_ context.Context, chatID int64, state string) error {
	if m.setStateErr != nil {
		return m.setStateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[chatID] = state
	return nil
}

func (m *mockState) ClearState(_ context.Context, chatID int64) error {
	if m.clearErr != nil {
		return m.clearErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, chatID)
	delete(m.pendingText, chatID)
	return nil
}

func (m *mockState) SetWaitingForTextState(_ context.Context, chatID int64) error {
	return m.SetState(context.Background(), chatID, "waiting_text")
}

func (m *mockState) SetWaitingForTimeState(_ context.Context, chatID int64) error {
	return m.SetState(context.Background(), chatID, "waiting_time")
}

func (m *mockState) SetWaitingRecurrenceState(_ context.Context, chatID int64) error {
	return m.SetState(context.Background(), chatID, "waiting_repeat")
}

func (m *mockState) SetWaitingWeekdaysState(_ context.Context, chatID int64, id int64) error {
	return m.SetState(context.Background(), chatID, "weekdays:")
}

func (m *mockState) SetWaitingTimezoneState(_ context.Context, chatID int64) error {
	return m.SetState(context.Background(), chatID, "waiting_timezone")
}

func (m *mockState) SetEditingState(_ context.Context, chatID int64, id int64) error {
	return m.SetState(context.Background(), chatID, "editing:"+formatInt(id))
}

func (m *mockState) SetRescheduleState(_ context.Context, chatID int64, id int64) error {
	return m.SetState(context.Background(), chatID, "reschedule:"+formatInt(id))
}

func (m *mockState) SetEditRepeatState(_ context.Context, chatID int64, id int64) error {
	return m.SetState(context.Background(), chatID, "edit_repeat:"+formatInt(id))
}

func (m *mockState) SetPendingText(_ context.Context, chatID int64, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingText[chatID] = text
	return nil
}

func (m *mockState) GetPendingText(_ context.Context, chatID int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pendingText[chatID], nil
}

func (m *mockState) SetPendingReminder(_ context.Context, chatID int64, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingID[chatID] = id
	return nil
}

func (m *mockState) SetSessionMessage(_ context.Context, chatID int64, msgID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionMsgs[chatID] = msgID
	return nil
}

func (m *mockState) GetSessionMessage(_ context.Context, chatID int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessionMsgs[chatID], nil
}

func (m *mockState) GetTimezone(_ context.Context, chatID int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.timezones[chatID], nil
}

func (m *mockState) SetTimezone(_ context.Context, chatID int64, tz string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timezones[chatID] = tz
	return nil
}

func (m *mockState) ParseEditingID(state string) (int64, bool) {
	return parseIDFromPrefix(state, "editing:")
}

func (m *mockState) ParseRescheduleID(state string) (int64, bool) {
	return parseIDFromPrefix(state, "reschedule:")
}

func (m *mockState) ParseEditRepeatID(state string) (int64, bool) {
	return parseIDFromPrefix(state, "edit_repeat:")
}

func (m *mockState) ParseWeekdaysID(state string) (int64, bool) {
	return parseIDFromPrefix(state, "weekdays:")
}

func (m *mockState) ParseCustomIntervalID(state string) (int64, bool) {
	return parseIDFromPrefix(state, "custom:")
}

func (m *mockState) ResolveReminderID(ctx context.Context, chatID int64, state string, prefixes ...string) int64 {
	for _, p := range prefixes {
		if id, ok := parseIDFromPrefix(state, p); ok {
			return id
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if id := m.pendingID[chatID]; id != 0 {
		return id
	}
	return 0
}

func (m *mockState) CleanupSessions(_ context.Context) error {
	return nil
}

// --- Mock Parser ---

type mockParser struct {
	timeResult     time.Time
	timeErr        error
	intervalResult string
	intervalErr    error
}

func newMockParser() *mockParser {
	return &mockParser{
		timeResult: time.Now().Add(1 * time.Hour),
	}
}

func (m *mockParser) ParseTime(_ string) (time.Time, error) {
	return m.timeResult, m.timeErr
}

func (m *mockParser) ParseInterval(_ string) (string, error) {
	return m.intervalResult, m.intervalErr
}

// --- Helpers ---

func formatInt(n int64) string {
	return strconv.FormatInt(n, 10)
}

func parseIDFromPrefix(state, prefix string) (int64, bool) {
	if !strings.HasPrefix(state, prefix) {
		return 0, false
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(state, prefix), 10, 64)
	return id, err == nil
}

// --- Mock Bot API ---

type mockBot struct {
	mu           sync.Mutex
	sentMessages []*telego.SendMessageParams
	editMessages []*telego.EditMessageTextParams
	delMessages  []*telego.DeleteMessageParams
	answeredCbs  []*telego.AnswerCallbackQueryParams

	sendErr error
	editErr error
	delErr  error
	ansErr  error
}

func newMockBot() *mockBot {
	return &mockBot{}
}

func (m *mockBot) SendMessage(ctx context.Context, params *telego.SendMessageParams) (*telego.Message, error) {
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	m.mu.Lock()
	m.sentMessages = append(m.sentMessages, params)
	m.mu.Unlock()
	return &telego.Message{MessageID: 100}, nil
}

func (m *mockBot) EditMessageText(ctx context.Context, params *telego.EditMessageTextParams) (*telego.Message, error) {
	if m.editErr != nil {
		return nil, m.editErr
	}
	m.mu.Lock()
	m.editMessages = append(m.editMessages, params)
	m.mu.Unlock()
	return &telego.Message{MessageID: params.MessageID}, nil
}

func (m *mockBot) DeleteMessage(ctx context.Context, params *telego.DeleteMessageParams) error {
	if m.delErr != nil {
		return m.delErr
	}
	m.mu.Lock()
	m.delMessages = append(m.delMessages, params)
	m.mu.Unlock()
	return nil
}

func (m *mockBot) AnswerCallbackQuery(ctx context.Context, params *telego.AnswerCallbackQueryParams) error {
	if m.ansErr != nil {
		return m.ansErr
	}
	m.mu.Lock()
	m.answeredCbs = append(m.answeredCbs, params)
	m.mu.Unlock()
	return nil
}

func (m *mockBot) GetChat(ctx context.Context, params *telego.GetChatParams) (*telego.ChatFullInfo, error) {
	return &telego.ChatFullInfo{Chat: telego.Chat{ID: params.ChatID.ID}}, nil
}

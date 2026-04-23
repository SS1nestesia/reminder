// Package telegram contains the Telegram-specific infrastructure:
// command and callback-query handlers, inline-keyboard builders, and
// a notifier that adapts [core.Notifier] to the Telegram Bot API.
//
// Business logic lives in the [core] package; this one is intentionally
// thin and focused on UX orchestration.
package telegram

// Callback data prefixes used in inline keyboard buttons.
// Centralizing these prevents typos and simplifies search/replace.
const (
	CBAddReminder   = "add_reminder"
	CBListReminders = "list_reminders"
	CBBackToMenu    = "back_to_menu"
	CBCancel        = "cancel"
	CBSetupTimezone = "setup_timezone"

	CBPrefixConfirmDelete = "confirm_delete:"
	CBPrefixDelete        = "delete:"
	CBPrefixEditText      = "edit_text:"
	CBPrefixEditTime      = "edit_time:"
	CBPrefixEditRepeat    = "edit_repeat:"
	CBPrefixView          = "view:"
	CBPrefixDone          = "done:"
	CBPrefixReschedule    = "reschedule:"
	CBPrefixSnoozeMenu    = "snooze_menu:"
	CBPrefixSnooze        = "snooze:"
	CBPrefixSnoozeBack    = "snooze_back:"
	CBPrefixQuick         = "quick:"
	CBPrefixRepeat        = "repeat:"
	CBPrefixWeekday       = "wd:"
	CBPrefixTimezone      = "tz:"

	// Friends system
	CBFriendsMenu         = "friends_menu"
	CBFriendsInvite       = "friends_invite"
	CBFriendReminders     = "friend_reminders"
	CBPrefixAcceptFriend  = "accept_friend:"
	CBPrefixRejectFriend  = "reject_friend:"
	CBPrefixRemoveFriend  = "remove_friend:"
	CBPrefixConfirmRemove = "confirm_remove_friend:"
	CBPrefixCreateFor     = "create_for:"
	CBCreateForSelf       = "create_for:self"
	CBPrefixFriendView    = "fview:"
	CBPrefixFriendDelete  = "fdelete:"
	CBPrefixFriendConfDel = "fconfirm_delete:"
)

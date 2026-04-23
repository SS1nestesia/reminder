package telegram

// UI message constants — centralized to avoid duplication and ease localization.
const (
	MsgMainMenu  = "👋 <b>Главное меню</b>\n\nВыберите действие:"
	MsgSaveError = "❌ <b>Ошибка при сохранении</b>"
	MsgNotFound  = "❌ Напоминание не найдено"
	MsgEmptyList = "📪 <b>У вас пока нет активных напоминаний.</b>\n\nНажмите ➕ Добавить, чтобы создать новое!"
	MsgCancelled = "✅ <b>Действие отменено</b>"

	// MsgTimeExamples lists supported phrasings for the "when" prompt.
	// Keep in sync with Parser.ParseTime fallback patterns.
	MsgTimeExamples = "Примеры:\n" +
		"• <code>через 30 минут</code>\n" +
		"• <code>через 2 часа 15 мин</code>\n" +
		"• <code>через 30 секунд</code>\n" +
		"• <code>+1ч</code> или <code>+30м</code>\n" +
		"• <code>завтра в 15:04</code>\n" +
		"• <code>сегодня в 22:30</code>\n" +
		"• <code>в пятницу 9:00</code>\n" +
		"• <code>25 марта в 14:30</code>"

	MsgAskTime = "⏰ <b>Когда напомнить?</b>\n\n" + MsgTimeExamples

	MsgTimeParseError = "❌ <b>Не удалось распознать время</b>\n\n" + MsgTimeExamples

	// MsgIntervalExamples lists supported phrasings for the "custom repeat" prompt.
	// Keep in sync with Parser.ParseInterval.
	MsgIntervalExamples = "Примеры интервала:\n" +
		"• <code>30 минут</code> / <code>30 мин</code> / <code>30м</code>\n" +
		"• <code>2 часа</code> / <code>2ч</code> / <code>2h</code>\n" +
		"• <code>3 дня</code> / <code>3д</code> / <code>3d</code>\n" +
		"• <code>1ч 30мин</code> или <code>1 час 30 минут</code>\n" +
		"• <code>полчаса</code>, <code>полдня</code>, <code>сутки</code>"

	MsgAskInterval = "⏱ <b>Каким будет интервал повтора?</b>\n\n" + MsgIntervalExamples

	MaxReminderTextLength = 1000
)

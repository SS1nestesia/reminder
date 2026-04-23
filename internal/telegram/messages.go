package telegram

// UI message constants — centralized to avoid duplication and ease localization.
const (
	MsgMainMenu  = "👋 <b>Главное меню</b>\n\nВыберите действие:"
	MsgSaveError = "❌ <b>Ошибка при сохранении</b>"
	MsgNotFound  = "❌ Напоминание не найдено"
	MsgEmptyList = "📪 <b>У вас пока нет активных напоминаний.</b>\n\nНажмите ➕ Добавить, чтобы создать новое!"
	MsgCancelled = "✅ <b>Действие отменено</b>"

	MaxReminderTextLength = 1000
)

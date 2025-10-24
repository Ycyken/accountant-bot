package telegram

import (
	"github.com/go-telegram/bot/models"
)

// mainMenuKeyboard returns main menu keyboard with quick actions
func mainMenuKeyboard() models.ReplyMarkup {
	return &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: "➕ Add expense"},
			},
			{
				{Text: "📂 Add category"},
				{Text: "📊 Statistics"},
			},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: false,
	}
}

// confirmKeyboard returns confirmation keyboard (Yes/No)
// TODO: Use when implementing confirmation dialogs
// nolint:unused
func confirmKeyboard(action string) models.ReplyMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Да", CallbackData: "confirm:" + action + ":yes"},
				{Text: "❌ Нет", CallbackData: "confirm:" + action + ":no"},
			},
		},
	}
}

// cancelKeyboard returns keyboard with cancel button
// TODO: Use when implementing multi-step operations
// nolint:unused
func cancelKeyboard() models.ReplyMarkup {
	return &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: "❌ Отменить"},
			},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}
}

// removeKeyboard returns markup to remove custom keyboard
// TODO: Use when removing custom keyboards
// nolint:unused
func removeKeyboard() models.ReplyMarkup {
	return &models.ReplyKeyboardRemove{
		RemoveKeyboard: true,
	}
}

// expenseConfirmKeyboard returns keyboard to confirm expense details
func expenseConfirmKeyboard() models.ReplyMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Подтвердить", CallbackData: "expense:confirm"},
				{Text: "❌ Отменить", CallbackData: "expense:cancel"},
			},
		},
	}
}

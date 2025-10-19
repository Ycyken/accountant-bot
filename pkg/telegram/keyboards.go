package telegram

import (
	"fmt"

	"saldo/pkg/db"

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

// emojiKeyboard returns keyboard with random emoji selection for category
func emojiKeyboard() models.ReplyMarkup {
	emojis := []string{
		"🍔", "🍕", "🍜", "🍱", "🍛", "🍗", "🥗", "🥙", // Food
		"🚗", "🚕", "🚌", "🚇", "🚊", "✈️", "🚲", "🛵", // Transport
		"🏠", "🏢", "🏪", "🏥", "💊", "🛒", "🎮", "🎬", // Other
		"💰", "💳", "💵", "📱", "💻", "⚡", "💡", "🔧",
	}

	// Create rows of 4 emojis each
	var rows [][]models.InlineKeyboardButton
	for i := 0; i < len(emojis); i += 4 {
		end := i + 4
		if end > len(emojis) {
			end = len(emojis)
		}

		var row []models.InlineKeyboardButton
		for _, emoji := range emojis[i:end] {
			row = append(row, models.InlineKeyboardButton{
				Text:         emoji,
				CallbackData: "emoji:" + emoji,
			})
		}
		rows = append(rows, row)
	}

	// Add cancel button
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "❌ Отменить", CallbackData: "emoji:cancel"},
	})

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

// userCategoriesKeyboard returns inline keyboard with user's categories
func userCategoriesKeyboard(categories []db.Category) models.ReplyMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(categories)+1)

	for _, cat := range categories {
		emoji := ""
		if cat.Emoji != nil {
			emoji = *cat.Emoji + " "
		}
		rows = append(rows, []models.InlineKeyboardButton{
			{
				Text:         emoji + cat.Title,
				CallbackData: fmt.Sprintf("select_cat:%d", cat.ID),
			},
		})
	}

	// Add "Create new category" button
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "➕ Создать новую категорию", CallbackData: "select_cat:new"},
	})

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
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

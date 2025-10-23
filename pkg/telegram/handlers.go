package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"saldo/pkg/services"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleStart handles /start command - registers or welcomes user
func (b *Bot) handleStart(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	user := update.Message.From
	chatID := update.Message.Chat.ID

	// Try to get or create user in database
	dbUser, err := b.getOrCreateUser(ctx, user)
	if err != nil {
		b.logger.Error(ctx, "failed to get or create user", "err", err, "telegram_user_id", user.ID)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Произошла ошибка при регистрации. Попробуйте позже.",
		})
		return
	}

	// Clear any previous state
	b.stateManager.ClearState(user.ID)

	welcomeText := fmt.Sprintf(
		"👋 Привет, %s!\n\n"+
			"Я помогу вам вести учет расходов.\n\n"+
			"Используйте кнопки ниже для управления:",
		user.FirstName,
	)

	b.logger.Print(ctx, "user started bot", "user_id", dbUser.ID, "telegram_user_id", user.ID, "username", user.Username)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        welcomeText,
		ReplyMarkup: mainMenuKeyboard(),
	})
}

// handleHelp handles /help command
func (b *Bot) handleHelp(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	helpText := `📚 <b>Справка по командам:</b>

<b>➕ Add expense</b> - Добавить новый расход
Нажмите кнопку и отправьте голосовое сообщение или текст с описанием расхода.

<b>📂 Add category</b> - Добавить категорию
Создайте новую категорию расходов с эмодзи.

<b>📊 Statistics</b> - Статистика
Показывает распределение расходов по категориям (в разработке).

<b>/cancel</b> - Отмена
Отменяет текущую операцию.

💡 <i>Совет:</i> Используйте кнопки меню для быстрого доступа к функциям.`

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        helpText,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: mainMenuKeyboard(),
	})
}

// handleCancel handles /cancel command
func (b *Bot) handleCancel(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	// Clear user state
	b.stateManager.ClearState(update.Message.From.ID)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        "✅ Операция отменена.",
		ReplyMarkup: mainMenuKeyboard(),
	})
}

// handleMessage handles text messages (keyboard buttons and state-based input)
func (b *Bot) handleMessage(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	// Get user from DB
	dbUser, err := b.getUserByTelegramID(ctx, userID)
	if err != nil || dbUser == nil {
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Пожалуйста, используйте /start для начала работы.",
		})
		return
	}

	// Check if this is a voice message
	if update.Message.Voice != nil {
		b.handleVoice(ctx, botAPI, update, dbUser)
		return
	}

	text := update.Message.Text

	// Check current state
	stateData := b.stateManager.GetState(userID)

	// Handle keyboard buttons
	switch text {
	case "➕ Add expense":
		b.handleAddExpenseStart(ctx, botAPI, chatID, userID)
		return
	case "📂 Add category":
		b.handleAddCategoryStart(ctx, botAPI, chatID, userID)
		return
	case "📊 Statistics":
		b.handleStatistics(ctx, botAPI, chatID)
		return
	}

	// Handle state-based input
	switch stateData.State {
	case StateAwaitingCategoryName:
		b.handleCategoryNameInput(ctx, botAPI, chatID, userID, text)
	case StateAwaitingExpense:
		b.handleExpenseTextInput(ctx, botAPI, chatID, userID, dbUser, text)
	case StateAwaitingDescription:
		b.handleDescriptionInput(ctx, botAPI, chatID, userID, dbUser, text)
	case StateIdle, StateAwaitingCategoryEmoji, StateAwaitingExpenseCategory:
		// These states are handled via callbacks, not text input
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Используйте кнопки меню или /help для списка команд.",
		})
	default:
		// Unknown message
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Используйте кнопки меню или /help для списка команд.",
		})
	}
}

// handleAddCategoryStart starts the add category flow
func (b *Bot) handleAddCategoryStart(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64) {
	b.stateManager.SetState(userID, StateAwaitingCategoryName)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      "📂 <b>Создание категории</b>\n\nВведите название категории:",
		ParseMode: models.ParseModeHTML,
	})
}

// handleCategoryNameInput handles category name input
func (b *Bot) handleCategoryNameInput(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, categoryName string) {
	// Save category name to state
	stateData := b.stateManager.GetState(userID)
	stateData.CategoryName = categoryName
	stateData.State = StateAwaitingCategoryEmoji
	b.stateManager.SetStateData(userID, stateData)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf("Отлично! Теперь выберите эмодзи для категории \"%s\":", categoryName),
		ReplyMarkup: emojiKeyboard(),
	})
}

// handleAddExpenseStart starts the add expense flow
func (b *Bot) handleAddExpenseStart(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64) {
	b.stateManager.SetState(userID, StateAwaitingExpense)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: "💰 <b>Добавление расхода</b>\n\n" +
			"Отправьте голосовое сообщение или напишите текстом.\n" +
			"Например: <code>500 рублей на еду в Макдональдс</code>\n\n" +
			"Используйте /cancel для отмены.",
		ParseMode: models.ParseModeHTML,
	})
}

// handleExpenseTextInput handles text input for expense
func (b *Bot) handleExpenseTextInput(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, user *User, text string) {
	// Get user categories
	saldoCategories, err := b.saldo.GetUserCategories(ctx, user.ID)
	if err != nil {
		b.logger.Error(ctx, "failed to get categories", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Ошибка получения категорий.",
		})
		return
	}

	categories := NewCategories(saldoCategories)

	// Extract category names
	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Title
	}

	// Parse expense using LLM
	parsed, err := b.llm.ParseExpense(ctx, text, categoryNames)
	if err != nil {
		b.logger.Error(ctx, "failed to parse expense", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Ошибка обработки текста.",
		})
		return
	}

	// Check if description is needed
	if parsed.NeedsDescription {
		// Save parsed data to state
		stateData := b.stateManager.GetState(userID)
		stateData.State = StateAwaitingDescription
		stateData.ExpenseData = &ExpenseData{
			Amount:   int(parsed.Amount * 100),
			Currency: parsed.Currency,
			Category: parsed.Category,
		}
		b.stateManager.SetStateData(userID, stateData)

		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "⚠️ Пожалуйста, добавьте описание расхода:",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Check if category is needed
	if parsed.NeedsCategory || parsed.Category == "" {
		// Save parsed data to state
		stateData := b.stateManager.GetState(userID)
		stateData.State = StateAwaitingExpenseCategory
		stateData.ExpenseData = &ExpenseData{
			Amount:      int(parsed.Amount * 100),
			Currency:    parsed.Currency,
			Description: parsed.Description,
		}
		b.stateManager.SetStateData(userID, stateData)

		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "📂 Выберите категорию:",
			ReplyMarkup: userCategoriesKeyboard(categories),
		})
		return
	}

	// Show confirmation
	b.showExpenseConfirmation(ctx, botAPI, chatID, userID, parsed)
}

// handleDescriptionInput handles description input
func (b *Bot) handleDescriptionInput(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, user *User, description string) {
	stateData := b.stateManager.GetState(userID)

	if stateData.ExpenseData == nil {
		b.stateManager.ClearState(userID)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Ошибка: данные расхода не найдены.",
		})
		return
	}

	stateData.ExpenseData.Description = description

	// Check if category is still needed
	if stateData.ExpenseData.Category == "" {
		// Get user categories
		saldoCategories, err := b.saldo.GetUserCategories(ctx, user.ID)
		if err != nil {
			b.logger.Error(ctx, "failed to get categories", "err", err)
			return
		}

		categories := NewCategories(saldoCategories)
		stateData.State = StateAwaitingExpenseCategory
		b.stateManager.SetStateData(userID, stateData)

		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "📂 Выберите категорию:",
			ReplyMarkup: userCategoriesKeyboard(categories),
		})
		return
	}

	// Create expense
	b.createExpense(ctx, botAPI, chatID, userID, user, stateData.ExpenseData)
}

// showExpenseConfirmation shows expense details for confirmation
func (b *Bot) showExpenseConfirmation(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, parsed *services.ParsedExpense) {
	// Save to state for confirmation
	stateData := b.stateManager.GetState(userID)
	stateData.ExpenseData = &ExpenseData{
		Amount:      int(parsed.Amount * 100),
		Currency:    parsed.Currency,
		Category:    parsed.Category,
		Description: parsed.Description,
	}
	b.stateManager.SetStateData(userID, stateData)

	text := "✅ <b>Подтвердите расход:</b>\n\n" + services.FormatExpenseDetails(parsed)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: expenseConfirmKeyboard(),
	})
}

// createExpense creates expense in database
func (b *Bot) createExpense(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, user *User, expenseData *ExpenseData) {
	// Create expense with category
	_, err := b.saldo.CreateExpenseWithCategory(
		ctx,
		user.ID,
		expenseData.Amount,
		expenseData.Currency,
		expenseData.Category,
		expenseData.Description,
	)
	if err != nil {
		b.logger.Error(ctx, "failed to create expense", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Ошибка сохранения расхода.",
		})
		return
	}

	// Clear state
	b.stateManager.ClearState(userID)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: fmt.Sprintf("✅ Расход добавлен!\n\n💰 %.2f %s\n📝 %s",
			float64(expenseData.Amount)/100,
			expenseData.Currency,
			expenseData.Description,
		),
		ReplyMarkup: mainMenuKeyboard(),
	})
}

// Download Telegram file by file ID
func (b *Bot) downloadTgFile(ctx context.Context, botAPI *bot.Bot, fileID string) (string, error) {
	file, err := botAPI.GetFile(ctx, &bot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		b.logger.Error(ctx, "failed to get file", "err", err)
		return "", err
	}

	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botAPI.Token(), file.FilePath)
	b.logger.Print(ctx, "file url", "url", fileURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		b.logger.Error(ctx, "failed to download file from telegram", "err", err)
		return "", err
	}
	defer resp.Body.Close()

	tmpOgg := fmt.Sprintf("/tmp/whisper/%s.ogg", fileID)
	err = os.MkdirAll(filepath.Dir(tmpOgg), 0755)
	if err != nil {
		return "", err
	}
	ogg, err := os.Create(tmpOgg)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(ogg, resp.Body)
	if err != nil {
		b.logger.Error(ctx, "failed to save downloaded file", "err", err)
		return "", err
	}
	ogg.Close()
	return tmpOgg, nil
}

// handleVoice handles voice messages
func (b *Bot) handleVoice(ctx context.Context, botAPI *bot.Bot, update *models.Update, user *User) {
	if update.Message == nil || update.Message.From == nil || update.Message.Voice == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	// Check if user is in expense flow
	stateData := b.stateManager.GetState(userID)
	if stateData.State != StateAwaitingExpense {
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Сначала нажмите '➕ Add expense' чтобы начать добавление расхода.",
		})
		return
	}

	voiceFileID := update.Message.Voice.FileID
	b.logger.Print(ctx, "received voice message", "file_id", voiceFileID)
	tmpOgg, err := b.downloadTgFile(ctx, botAPI, voiceFileID)
	defer os.Remove(tmpOgg)
	if err != nil {
		b.logger.Error(ctx, "failed to download voice file", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Ошибка получения голосового сообщения.",
		})
		return
	}

	// Mock transcription
	transcription, err := b.whisper.Transcribe(ctx, tmpOgg)
	b.logger.Print(ctx, "transcription result", "text", transcription)
	if err != nil {
		b.logger.Error(ctx, "failed to transcribe voice", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Ошибка распознавания голоса.",
		})
		return
	}

	// Process transcription as text
	b.handleExpenseTextInput(ctx, botAPI, chatID, userID, user, transcription)
}

// handleStatistics handles statistics request
func (b *Bot) handleStatistics(ctx context.Context, botAPI *bot.Bot, chatID int64) {
	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      "📊 <b>Статистика</b>\n\n<i>Функция в разработке...</i>",
		ParseMode: models.ParseModeHTML,
	})
}

// handleCallback handles callback queries from inline keyboards
func (b *Bot) handleCallback(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	callback := update.CallbackQuery
	data := callback.Data
	userID := callback.From.ID

	// Extract chatID from callback message
	var chatID int64
	if msg := callback.Message.Message; msg != nil {
		chatID = msg.Chat.ID
	} else {
		b.logger.Error(ctx, "callback message is nil")
		return
	}

	b.logger.Print(ctx, "callback received", "data", data, "from", callback.From.Username)

	// Get user from DB
	user, err := b.getUserByTelegramID(ctx, userID)
	if err != nil || user == nil {
		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Ошибка: пользователь не найден",
			ShowAlert:       true,
		})
		return
	}

	// Parse callback data
	parts := strings.SplitN(data, ":", 2)
	if len(parts) < 2 {
		return
	}

	action := parts[0]
	value := parts[1]

	switch action {
	case "emoji":
		b.handleEmojiSelection(ctx, botAPI, callback, chatID, userID, user, value)
	case "select_cat":
		b.handleCategorySelection(ctx, botAPI, callback, chatID, userID, user, value)
	case "expense":
		b.handleExpenseAction(ctx, botAPI, callback, chatID, userID, user, value)
	default:
		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Неизвестное действие",
		})
	}
}

// handleEmojiSelection handles emoji selection for category
func (b *Bot) handleEmojiSelection(ctx context.Context, botAPI *bot.Bot, callback *models.CallbackQuery, chatID int64, userID int64, user *User, emoji string) {
	if emoji == "cancel" {
		b.stateManager.ClearState(userID)
		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Отменено.",
			ReplyMarkup: mainMenuKeyboard(),
		})
		return
	}

	stateData := b.stateManager.GetState(userID)
	if stateData.State != StateAwaitingCategoryEmoji || stateData.CategoryName == "" {
		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Ошибка: нет данных категории",
			ShowAlert:       true,
		})
		return
	}

	// Create category
	_, err := b.saldo.CreateCategory(ctx, user.ID, stateData.CategoryName, &emoji)
	if err != nil {
		b.logger.Error(ctx, "failed to create category", "err", err)
		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Ошибка создания категории",
			ShowAlert:       true,
		})
		return
	}

	// Clear state
	b.stateManager.ClearState(userID)

	_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
		Text:            "Категория создана!",
	})

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: fmt.Sprintf("✅ Категория создана!\n\n%s %s",
			emoji,
			stateData.CategoryName,
		),
		ReplyMarkup: mainMenuKeyboard(),
	})
}

// handleCategorySelection handles category selection for expense
func (b *Bot) handleCategorySelection(ctx context.Context, botAPI *bot.Bot, callback *models.CallbackQuery, chatID int64, userID int64, user *User, value string) {
	if value == "new" {
		// Start new category flow
		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		b.handleAddCategoryStart(ctx, botAPI, chatID, userID)
		return
	}

	// Parse category ID
	categoryID, err := strconv.Atoi(value)
	if err != nil {
		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Ошибка: неверный ID категории",
			ShowAlert:       true,
		})
		return
	}

	// Get category
	saldoCategory, err := b.saldo.GetCategoryByID(ctx, categoryID)
	if err != nil || saldoCategory == nil {
		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Ошибка: категория не найдена",
			ShowAlert:       true,
		})
		return
	}

	stateData := b.stateManager.GetState(userID)
	if stateData.ExpenseData == nil {
		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Ошибка: нет данных расхода",
			ShowAlert:       true,
		})
		return
	}

	// Update expense data with category
	category := NewCategory(saldoCategory)
	stateData.ExpenseData.Category = category.Title

	// Create expense
	b.createExpense(ctx, botAPI, chatID, userID, user, stateData.ExpenseData)

	_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})
}

// handleExpenseAction handles expense confirmation/cancellation
func (b *Bot) handleExpenseAction(ctx context.Context, botAPI *bot.Bot, callback *models.CallbackQuery, chatID int64, userID int64, user *User, action string) {
	if action == "cancel" {
		b.stateManager.ClearState(userID)
		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Отменено.",
			ReplyMarkup: mainMenuKeyboard(),
		})
		return
	}

	if action == "confirm" {
		stateData := b.stateManager.GetState(userID)
		if stateData.ExpenseData == nil {
			_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: callback.ID,
				Text:            "Ошибка: нет данных расхода",
				ShowAlert:       true,
			})
			return
		}

		b.createExpense(ctx, botAPI, chatID, userID, user, stateData.ExpenseData)

		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Расход сохранен!",
		})
	}
}

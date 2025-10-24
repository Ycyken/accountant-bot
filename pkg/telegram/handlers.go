package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	case "📊 Statistics":
		b.handleStatistics(ctx, botAPI, chatID)
		return
	}

	// Handle state-based input
	switch stateData.State {
	case StateAwaitingExpense:
		b.handleExpenseTextInput(ctx, botAPI, chatID, userID, dbUser, text)
	case StateIdle:
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
	expenses, err := b.llm.ParseExpenses(ctx, text, categoryNames)
	if err != nil {
		b.logger.Error(ctx, "failed to parse expense", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Ошибка обработки текста.",
		})
		return
	}

	if len(expenses) == 0 {
		b.logger.Print(ctx, "пользователь ввёл сообщение без расходов", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Не получилось получить расходы.",
		})
		return
	}

	// Show confirmation
	b.showExpenseConfirmation(ctx, botAPI, chatID, userID, expenses)
}

// showExpenseConfirmation shows expense details for confirmation
func (b *Bot) showExpenseConfirmation(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, expenses []services.ParsedExpense) {
	// Save to state for confirmation
	stateData := b.stateManager.GetState(userID)
	stateData.ExpensesData = make([]ExpenseData, len(expenses))

	for i, exp := range expenses {
		stateData.ExpensesData[i] = ExpenseData{
			Amount:      int(exp.Amount * 100),
			Currency:    exp.Currency,
			Category:    exp.Category,
			Description: exp.Description,
		}
	}
	b.stateManager.SetStateData(userID, stateData)

	text := "✅ <b>Подтвердите расходы:</b>\n\n" + services.FormatExpenseDetails(expenses)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: expenseConfirmKeyboard(),
	})
}

// createExpense creates expenses in database
func (b *Bot) createExpenses(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, user *User, expenses []ExpenseData) {
	// Create expense with category
	for _, exp := range expenses {
		_, err := b.saldo.CreateExpenseWithCategory(
			ctx,
			user.ID,
			exp.Amount,
			exp.Currency,
			exp.Category,
			exp.Description,
		)
		if err != nil {
			b.logger.Error(ctx, "failed to create expense", "err", err)
			_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Ошибка сохранения расхода.",
			})
			return
		}
	}

	// Clear state
	b.stateManager.ClearState(userID)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "✅ Расходы добавлены!\n\n💰",
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
	if err != nil {
		b.logger.Error(ctx, "failed to download voice file", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Ошибка получения голосового сообщения.",
		})
		return
	}
	defer os.Remove(tmpOgg)

	// Mock transcription
	transcription, err := b.transcriber.Transcribe(ctx, tmpOgg)
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
	case "expense":
		b.handleExpenseAction(ctx, botAPI, callback, chatID, userID, user, value)
	default:
		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Неизвестное действие",
		})
	}
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
		if stateData.ExpensesData == nil {
			_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: callback.ID,
				Text:            "Ошибка: нет данных расхода",
				ShowAlert:       true,
			})
			return
		}

		b.createExpenses(ctx, botAPI, chatID, userID, user, stateData.ExpensesData)

		_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Расход сохранен!",
		})
	}
}

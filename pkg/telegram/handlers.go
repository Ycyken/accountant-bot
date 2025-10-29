package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"saldo/pkg/services"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleStart handles /start command - registers or welcomes user
func (b *Bot) handleStart(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	commandsProcessed.WithLabelValues("start").Inc()
	if update.Message == nil || update.Message.From == nil {
		return
	}

	user := update.Message.From
	chatID := update.Message.Chat.ID

	// Try to get or create user in database
	dbUser, err := b.getOrCreateUser(ctx, user)
	if err != nil {
		errorsTotal.WithLabelValues("user_registration").Inc()
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
	commandsProcessed.WithLabelValues("help").Inc()
	if update.Message == nil || update.Message.From == nil {
		return
	}

	helpText := `📚 <b>Справка по командам:</b>

<b>➕ Добавить расход</b> - Добавить новый расход
Нажмите кнопку и отправьте голосовое сообщение или текст с описанием расхода.

<b>📂 Добавить категорию</b> - Добавить свою категорию расхода
Создайте новую категорию расходов с эмодзи. (пока не реализовано)

<b>📊 Статистика</b> - Статистика
Показать распределение расходов по категориям или тратам.

💡 Используйте кнопки меню для доступа к функциям.`

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        helpText,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: mainMenuKeyboard(),
	})
}

// handleMessage handles text messages (keyboard buttons and state-based input)
func (b *Bot) handleMessage(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	messagesProcessed.WithLabelValues("text").Inc()
	if update.Message == nil || update.Message.From == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	// Get user from DB
	dbUser, err := b.getUserByTelegramID(ctx, userID)
	if err != nil || dbUser == nil {
		errorsTotal.WithLabelValues("user_not_found").Inc()
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Пожалуйста, используйте /start для начала работы.",
		})
		return
	}

	text := update.Message.Text

	// Check current state
	stateData := b.stateManager.GetState(userID)

	// Check if this is a voice message
	if update.Message.Voice != nil {
		// If awaiting custom period, reject voice input
		if stateData.State == StateAwaitingCustomPeriod {
			_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Пожалуйста, введите период текстом в формате: ДД.ММ.ГГ ДД.ММ.ГГ",
			})
			return
		}
		// Clear any pending expense state and process voice as new expense
		if stateData.ExpensesData != nil {
			b.stateManager.ClearState(userID)
		}
		b.handleVoice(ctx, botAPI, update, dbUser)
		return
	}

	// Handle keyboard buttons
	switch text {
	case "➕ Добавить расход":
		buttonsPressed.WithLabelValues("add_expense").Inc()
		b.handleAddExpenseStart(ctx, botAPI, chatID, userID)
		return
	case "📂 Добавить категорию":
		buttonsPressed.WithLabelValues("add_category").Inc()
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "⚠️ Кастомные категории ещё не реализованы.",
			ReplyMarkup: mainMenuKeyboard(),
		})
		return
	case "📊 Статистика":
		buttonsPressed.WithLabelValues("statistics").Inc()
		b.handleStatistics(ctx, botAPI, chatID, userID, dbUser)
		return
	case "💰 Траты за неделю":
		buttonsPressed.WithLabelValues("week_expenses").Inc()
		period := GetWeekPeriod()
		b.handleStatisticsByExpenses(ctx, botAPI, chatID, userID, dbUser, period)
		return
	case "🔙 Назад":
		buttonsPressed.WithLabelValues("back").Inc()
		b.handleBack(ctx, botAPI, chatID, userID, stateData)
		return
	case "🔙 К статистике":
		buttonsPressed.WithLabelValues("to_statistics").Inc()
		b.handleStatistics(ctx, botAPI, chatID, userID, dbUser)
		return
	case "📊 По категориям":
		buttonsPressed.WithLabelValues("by_categories").Inc()
		b.handleStatsTypeSelection(ctx, botAPI, chatID, userID, "categories")
		return
	case "💸 По тратам":
		buttonsPressed.WithLabelValues("by_expenses").Inc()
		b.handleStatsTypeSelection(ctx, botAPI, chatID, userID, "expenses")
		return
	case "📅 За сегодня":
		buttonsPressed.WithLabelValues("period_today").Inc()
		b.handlePeriodSelection(ctx, botAPI, chatID, userID, dbUser, stateData, "today")
		return
	case "📅 За неделю":
		buttonsPressed.WithLabelValues("period_week").Inc()
		b.handlePeriodSelection(ctx, botAPI, chatID, userID, dbUser, stateData, "week")
		return
	case "📅 За месяц":
		buttonsPressed.WithLabelValues("period_month").Inc()
		b.handlePeriodSelection(ctx, botAPI, chatID, userID, dbUser, stateData, "month")
		return
	case "📅 За всё время":
		buttonsPressed.WithLabelValues("period_alltime").Inc()
		b.handlePeriodSelection(ctx, botAPI, chatID, userID, dbUser, stateData, "alltime")
		return
	case "📅 Кастомный период":
		buttonsPressed.WithLabelValues("period_custom").Inc()
		b.handleCustomPeriodStart(ctx, botAPI, chatID, userID, stateData)
		return
	}

	// Check if user is entering custom period
	if stateData.State == StateAwaitingCustomPeriod {
		b.handleCustomPeriodInput(ctx, botAPI, chatID, userID, dbUser, text)
		return
	}

	// Clear any pending expense state and treat message as new expense input
	if stateData.ExpensesData != nil {
		b.stateManager.ClearState(userID)
	}

	// Any other text message is treated as expense input
	b.handleExpenseTextInput(ctx, botAPI, chatID, userID, dbUser, text)
}

// handleAddExpenseStart starts the add expense flow
func (b *Bot) handleAddExpenseStart(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64) {
	b.stateManager.SetState(userID, StateAwaitingExpense)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: "💰 <b>Добавление расхода</b>\n\n" +
			"Отправьте голосовое сообщение или напишите текстом.\n" +
			"Например: <code>500 рублей на еду в Макдональдс</code>",
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: mainMenuKeyboard(),
	})
}

// handleExpenseTextInput handles text input for expense
func (b *Bot) handleExpenseTextInput(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, user *User, text string) {
	// Get user categories
	saldoCategories, err := b.saldo.GetUserCategories(ctx, user.ID)
	if err != nil {
		errorsTotal.WithLabelValues("get_categories").Inc()
		b.logger.Error(ctx, "failed to get categories", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Ошибка получения категорий.",
			ReplyMarkup: mainMenuKeyboard(),
		})
		return
	}

	categories := NewCategories(saldoCategories)

	// Extract category names
	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Title
	}

	// Parse expense using LLM with timing
	startTime := time.Now()
	expenses, err := b.llm.ParseExpenses(ctx, text, categoryNames)
	llmParseDuration.Observe(time.Since(startTime).Seconds())

	if err != nil {
		errorsTotal.WithLabelValues("llm_parse").Inc()
		b.logger.Error(ctx, "failed to parse expense", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Ошибка обработки текста.",
			ReplyMarkup: mainMenuKeyboard(),
		})
		return
	}

	if len(expenses) == 0 {
		errorsTotal.WithLabelValues("llm_parse_failed").Inc()
		b.logger.Print(ctx, "пользователь ввёл сообщение без расходов", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Не получилось получить расходы.",
			ReplyMarkup: mainMenuKeyboard(),
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
	// Get existing categories to track new ones
	existingCategories, _ := b.saldo.GetUserCategories(ctx, user.ID)
	existingCategoryMap := make(map[string]bool)
	for _, cat := range existingCategories {
		existingCategoryMap[cat.Title] = true
	}

	// Create expense with category
	for _, exp := range expenses {
		// Track if category is new
		categoryIsNew := exp.Category != "" && !existingCategoryMap[exp.Category]

		_, err := b.saldo.CreateExpenseWithCategory(
			ctx,
			user.ID,
			exp.Amount,
			exp.Currency,
			exp.Category,
			exp.Description,
		)
		if err != nil {
			errorsTotal.WithLabelValues("database").Inc()
			b.logger.Error(ctx, "failed to create expense", "err", err)
			_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Ошибка сохранения расхода.",
			})
			return
		}

		expensesCreated.Inc()

		// Increment category counter if new category was created
		if categoryIsNew {
			categoriesCreated.Inc()
			existingCategoryMap[exp.Category] = true // Mark as existing for next expense
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
	messagesProcessed.WithLabelValues("voice").Inc()
	if update.Message == nil || update.Message.From == nil || update.Message.Voice == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	voiceFileID := update.Message.Voice.FileID
	b.logger.Print(ctx, "received voice message", "file_id", voiceFileID)
	tmpOgg, err := b.downloadTgFile(ctx, botAPI, voiceFileID)
	if err != nil {
		errorsTotal.WithLabelValues("download_file").Inc()
		b.logger.Error(ctx, "failed to download voice file", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Ошибка получения голосового сообщения.",
			ReplyMarkup: mainMenuKeyboard(),
		})
		return
	}
	defer os.Remove(tmpOgg)

	// Transcribe voice message with timing
	startTime := time.Now()
	transcription, err := b.transcriber.Transcribe(ctx, tmpOgg)
	transcriptionDuration.Observe(time.Since(startTime).Seconds())

	b.logger.Print(ctx, "transcription result", "text", transcription)
	if err != nil {
		errorsTotal.WithLabelValues("transcription").Inc()
		b.logger.Error(ctx, "failed to transcribe voice", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Ошибка распознавания голоса.",
			ReplyMarkup: mainMenuKeyboard(),
		})
		return
	}

	// Process transcription as text
	b.handleExpenseTextInput(ctx, botAPI, chatID, userID, user, transcription)
}

// handleStatistics shows statistics menu
func (b *Bot) handleStatistics(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, _ *User) {
	stateData := b.stateManager.GetState(userID)
	stateData.State = StateInStatsMenu
	stateData.InStatsFlow = true
	b.stateManager.SetStateData(userID, stateData)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "📊 <b>Выберите тип статистики:</b>",
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: statisticsMenuKeyboard(),
	})
}

// handleStatsTypeSelection handles statistics type selection from reply keyboard
func (b *Bot) handleStatsTypeSelection(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, statsType string) {
	stateData := b.stateManager.GetState(userID)
	stateData.State = StateInPeriodSelection
	stateData.StatsType = statsType
	stateData.InStatsFlow = true
	b.stateManager.SetStateData(userID, stateData)

	var text string
	includeAllTime := false
	if statsType == "categories" {
		text = "📊 <b>Статистика по категориям</b>\n\nВыберите период:"
		includeAllTime = true
	} else {
		text = "💸 <b>Статистика по тратам</b>\n\nВыберите период:"
	}

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: periodSelectionKeyboard(includeAllTime),
	})
}

// handlePeriodSelection handles period selection from reply keyboard
func (b *Bot) handlePeriodSelection(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, user *User, stateData *UserStateData, periodType string) {
	// Get period
	var period TimePeriod
	switch periodType {
	case "today":
		period = GetTodayPeriod()
	case "week":
		period = GetWeekPeriod()
	case "month":
		period = GetMonthPeriod()
	case "alltime":
		period = GetAllTimePeriod()
	default:
		return
	}

	statsType := stateData.StatsType

	// Show statistics with appropriate keyboard
	if statsType == "categories" {
		b.handleStatisticsByCategories(ctx, botAPI, chatID, userID, user, period)
	} else if statsType == "expenses" {
		b.handleStatisticsByExpenses(ctx, botAPI, chatID, userID, user, period)
	}
}

// handleCustomPeriodStart starts custom period input
func (b *Bot) handleCustomPeriodStart(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, stateData *UserStateData) {
	stateData.State = StateAwaitingCustomPeriod
	b.stateManager.SetStateData(userID, stateData)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: "📅 <b>Введите кастомный период</b>\n\n" +
			"Форматы:\n" +
			"• <code>03.04.25 07.04.25</code>\n" +
			"• <code>03.04.25 - 07.04.25</code>\n" +
			"• <code>03.04 07.04</code> (текущий год)\n" +
			"• <code>03.04 - 07.04</code> (текущий год)",
		ParseMode: models.ParseModeHTML,
	})
}

// handleBack handles back button navigation
func (b *Bot) handleBack(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, stateData *UserStateData) {
	switch stateData.State {
	case StateInPeriodSelection:
		// Go back to stats menu
		b.handleStatistics(ctx, botAPI, chatID, userID, nil)
	case StateInStatsMenu:
		// Go back to main menu
		b.stateManager.ClearState(userID)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Главное меню:",
			ReplyMarkup: mainMenuKeyboard(),
		})
	default:
		// Default to main menu
		b.stateManager.ClearState(userID)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Главное меню:",
			ReplyMarkup: mainMenuKeyboard(),
		})
	}
}

// handleStatisticsByCategories handles statistics by categories request with period
func (b *Bot) handleStatisticsByCategories(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, user *User, period TimePeriod) {
	// Get all expenses for user
	expenses, err := b.saldo.GetUserExpenses(ctx, user.ID)
	if err != nil {
		errorsTotal.WithLabelValues("database").Inc()
		b.logger.Error(ctx, "failed to get expenses", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Ошибка получения расходов.",
		})
		return
	}

	// Convert to telegram expenses
	allExpenses := NewExpenses(expenses)

	// Filter expenses by period
	var tgExpenses []Expense
	for _, exp := range allExpenses {
		if exp.CreatedAt.After(period.Start) && exp.CreatedAt.Before(period.End) {
			tgExpenses = append(tgExpenses, exp)
		}
	}

	// Group expenses by category and currency
	type CategoryStats struct {
		Title   string
		Emoji   string
		Amounts map[string]int // currency -> amount in cents
	}

	categoryMap := make(map[string]*CategoryStats)

	// Track currency frequency (count of occurrences)
	currencyFrequency := make(map[string]int)

	for _, exp := range tgExpenses {
		var categoryKey, categoryTitle, emoji string

		if exp.Category != nil {
			categoryKey = exp.Category.Title
			categoryTitle = exp.Category.Title
			emoji = exp.Category.Emoji
		} else {
			categoryKey = "__no_category__"
			categoryTitle = "Без категории"
			emoji = "❓"
		}

		// Initialize category if not exists
		if _, exists := categoryMap[categoryKey]; !exists {
			categoryMap[categoryKey] = &CategoryStats{
				Title:   categoryTitle,
				Emoji:   emoji,
				Amounts: make(map[string]int),
			}
		}

		// Track currency frequency
		currencyFrequency[exp.Currency]++

		// Add amount to category
		categoryMap[categoryKey].Amounts[exp.Currency] += exp.Amount
	}

	// Sort currencies by frequency (most frequent first)
	type currencyWithFreq struct {
		currency  string
		frequency int
	}
	currenciesWithFreq := make([]currencyWithFreq, 0, len(currencyFrequency))
	for currency, freq := range currencyFrequency {
		currenciesWithFreq = append(currenciesWithFreq, currencyWithFreq{currency, freq})
	}
	sort.Slice(currenciesWithFreq, func(i, j int) bool {
		return currenciesWithFreq[i].frequency > currenciesWithFreq[j].frequency
	})

	// Extract sorted currency order
	currencyOrder := make([]string, len(currenciesWithFreq))
	for i, cf := range currenciesWithFreq {
		currencyOrder[i] = cf.currency
	}

	// Format statistics message
	if len(categoryMap) == 0 {
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "📊 <b>Статистика</b>\n\n<i>Пока нет расходов.</i>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Sort categories by total amount (with currency rate approximation)
	type categoryWithTotal struct {
		stats *CategoryStats
		total int
	}
	categoriesWithTotal := make([]categoryWithTotal, 0, len(categoryMap))
	for _, stats := range categoryMap {
		total := 0
		for currency, amountCents := range stats.Amounts {
			curr := strings.ToUpper(currency)
			if curr == "USD" || curr == "EUR" || curr == "GBP" || curr == "CHF" {
				total += amountCents * 100
			} else if curr == "GEL" {
				total += amountCents * 30
			} else if curr == "CNY" {
				total += amountCents * 10
			} else if curr == "JPY" {
				total += amountCents / 2
			} else if curr == "KZT" {
				total += amountCents / 7
			} else {
				total += amountCents
			}
		}
		categoriesWithTotal = append(categoriesWithTotal, categoryWithTotal{stats, total})
	}
	sort.Slice(categoriesWithTotal, func(i, j int) bool {
		return categoriesWithTotal[i].total > categoriesWithTotal[j].total
	})

	text := "📊 <b>Статистика по категориям:</b>\n"
	text += fmt.Sprintf("<i>%s</i>\n\n", FormatPeriod(period))

	// Format each category (sorted by total)
	for _, cat := range categoriesWithTotal {
		stats := cat.stats
		text += fmt.Sprintf("%s <b>%s:</b> ", stats.Emoji, stats.Title)

		// Format amounts by currency in order of frequency
		first := true
		for _, currency := range currencyOrder {
			// Only show currencies that exist in this category
			amountCents, exists := stats.Amounts[currency]
			if !exists {
				continue
			}

			if !first {
				text += "/"
			}
			first = false

			// Convert cents to main units
			amount := float64(amountCents) / 100.0

			// Get currency symbol
			currencySymbol := getCurrencySymbol(currency)
			text += fmt.Sprintf("%.2f%s", amount, currencySymbol)
		}

		text += "\n"
	}

	// Update state to stats menu after showing results
	stateData := b.stateManager.GetState(userID)
	stateData.State = StateInStatsMenu
	stateData.InStatsFlow = true
	b.stateManager.SetStateData(userID, stateData)

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: statisticsMenuKeyboard(),
	})
}

// getCurrencySymbol returns the symbol for a currency code
func getCurrencySymbol(currency string) string {
	switch strings.ToUpper(currency) {
	case "RUB":
		return "₽"
	case "USD":
		return "$"
	case "EUR":
		return "€"
	case "GEL":
		return "₾"
	case "GBP":
		return "£"
	case "JPY":
		return "¥"
	case "CNY":
		return "¥"
	case "CHF":
		return "₣"
	default:
		return currency
	}
}

// handleStatisticsByExpenses handles statistics by individual expenses with period
func (b *Bot) handleStatisticsByExpenses(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, user *User, period TimePeriod) {
	// Get all expenses for user
	expenses, err := b.saldo.GetUserExpenses(ctx, user.ID)
	if err != nil {
		errorsTotal.WithLabelValues("database").Inc()
		b.logger.Error(ctx, "failed to get expenses", "err", err)
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Ошибка получения расходов.",
		})
		return
	}

	// Convert to telegram expenses
	allExpenses := NewExpenses(expenses)

	// Filter expenses by period
	var tgExpenses []Expense
	for _, exp := range allExpenses {
		if exp.CreatedAt.After(period.Start) && exp.CreatedAt.Before(period.End) {
			tgExpenses = append(tgExpenses, exp)
		}
	}

	// Check if we're in stats flow (from stats menu) or from main menu (week expenses)
	stateData := b.stateManager.GetState(userID)
	var replyMarkup models.ReplyMarkup

	if stateData.InStatsFlow {
		// User came from stats menu, show statistics menu
		replyMarkup = statisticsMenuKeyboard()
		// Update state to stats menu after showing results
		stateData.State = StateInStatsMenu
		b.stateManager.SetStateData(userID, stateData)
	} else {
		// User came from main menu (week expenses), keep main menu
		replyMarkup = mainMenuKeyboard()
		// Clear state to ensure back button goes to main menu
		b.stateManager.ClearState(userID)
	}

	if len(tgExpenses) == 0 {
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        fmt.Sprintf("📊 <b>Статистика по тратам:</b>\n<i>%s</i>\n\n<i>Нет расходов за этот период.</i>", FormatPeriod(period)),
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: replyMarkup,
		})
		return
	}

	// Sort by date (newest first)
	sort.Slice(tgExpenses, func(i, j int) bool {
		return tgExpenses[i].CreatedAt.After(tgExpenses[j].CreatedAt)
	})

	text := "📊 <b>Статистика по тратам:</b>\n"
	text += fmt.Sprintf("<i>%s</i>\n\n", FormatPeriod(period))

	// Format each expense
	for _, exp := range tgExpenses {
		// Format: Description(Category): Amount (Date) or Category: Amount (Date) if no description
		categoryName := "Без категории"
		emoji := "❓"
		if exp.Category != nil {
			categoryName = exp.Category.Title
			emoji = exp.Category.Emoji
		}

		amount := float64(exp.Amount) / 100.0
		currencySymbol := getCurrencySymbol(exp.Currency)
		dateStr := FormatDate(exp.CreatedAt)

		if exp.Description != "" {
			// Capitalize first letter of description
			description := exp.Description
			if len(description) > 0 {
				runes := []rune(description)
				runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
				description = string(runes)
			}
			text += fmt.Sprintf("<b>%s</b> (%s%s): %.2f%s (%s)\n",
				description, emoji, categoryName, amount, currencySymbol, dateStr)
		} else {
			text += fmt.Sprintf("<b>%s%s</b>: %.2f%s (%s)\n",
				emoji, categoryName, amount, currencySymbol, dateStr)
		}
	}

	_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: replyMarkup,
	})
}

// handleCustomPeriodInput handles custom period input from user
func (b *Bot) handleCustomPeriodInput(ctx context.Context, botAPI *bot.Bot, chatID int64, userID int64, user *User, text string) {
	// Parse custom period
	period, err := ParseCustomPeriod(text)
	if err != nil {
		// Keep period selection menu on error
		stateData := b.stateManager.GetState(userID)
		includeAllTime := stateData.StatsType == "categories"

		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        fmt.Sprintf("❌ Ошибка: %v\n\nПожалуйста, введите период в формате:\n• ДД.ММ.ГГ ДД.ММ.ГГ\n• ДД.ММ - ДД.ММ (текущий год)", err),
			ReplyMarkup: periodSelectionKeyboard(includeAllTime),
		})
		return
	}

	// Get state to know which stats type was requested
	stateData := b.stateManager.GetState(userID)
	statsType := stateData.StatsType

	// For expenses, check max period is 1 month
	if statsType == "expenses" && period.DaysBetween() > 31 {
		_, _ = botAPI.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Для статистики по тратам период не может быть больше месяца (31 день).",
			ReplyMarkup: periodSelectionKeyboard(false), // expenses don't have all-time
		})
		return
	}

	// Keep InStatsFlow flag to show stats menu after results
	stateData.State = StateInStatsMenu
	b.stateManager.SetStateData(userID, stateData)

	// Show statistics
	if statsType == "categories" {
		b.handleStatisticsByCategories(ctx, botAPI, chatID, userID, user, period)
	} else {
		b.handleStatisticsByExpenses(ctx, botAPI, chatID, userID, user, period)
	}
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
		errorsTotal.WithLabelValues("user_not_found").Inc()
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
		callbacksProcessed.WithLabelValues("cancel").Inc()
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
		callbacksProcessed.WithLabelValues("confirm").Inc()
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

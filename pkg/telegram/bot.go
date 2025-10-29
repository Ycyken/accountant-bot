package telegram

import (
	"context"
	"errors"
	"fmt"
	"time"

	"saldo/pkg/saldo"
	"saldo/pkg/services"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/vmkteam/embedlog"
)

type Bot struct {
	api              *bot.Bot
	logger           embedlog.Logger
	saldo            *saldo.Manager
	debug            bool
	stateManager     *StateManager
	transcriber      services.Transcriber
	llm              services.LLM
	prometheusClient *services.PrometheusClient
}

type Config struct {
	Token     string
	Debug     bool
	GroqToken string
}

// New creates a new Telegram bot instance
func New(ctx context.Context, cfg Config, saldoService *saldo.Manager, logger embedlog.Logger) (*Bot, error) {
	if cfg.Token == "" {
		return nil, errors.New("telegram token is required")
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler(logger)),
	}

	if cfg.Debug {
		opts = append(opts, bot.WithDebug())
	}

	api, err := bot.New(cfg.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	groq := saldo.NewGroq(cfg.GroqToken)

	// Create Prometheus client for metric restoration
	// URL: http://prometheus:9090 for Docker, http://localhost:9090 for local dev
	prometheusURL := "http://prometheus:9090"
	promClient, err := services.NewPrometheusClient(prometheusURL, logger)
	if err != nil {
		logger.Error(ctx, "failed to create prometheus client", "err", err)
		// Don't fail bot startup, will retry later
		promClient = nil
	}

	b := &Bot{
		api:              api,
		logger:           logger,
		saldo:            saldoService,
		debug:            cfg.Debug,
		stateManager:     NewStateManager(),
		transcriber:      groq,
		llm:              groq,
		prometheusClient: promClient,
	}

	// Register command handlers
	b.registerHandlers()

	// Initialize metrics from database and Prometheus
	b.initializeMetrics(ctx)

	return b, nil
}

// Start starts the bot with long polling
func (b *Bot) Start(ctx context.Context) error {
	me, err := b.api.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}

	b.logger.Print(ctx, "telegram bot started", "username", me.Username, "id", me.ID)
	b.api.Start(ctx)

	return nil
}

// Stop gracefully stops the bot
func (b *Bot) Stop(ctx context.Context) {
	b.logger.Print(ctx, "stopping telegram bot")
}

// initializeMetrics initializes Prometheus metrics from Prometheus and database
// This ensures metrics persist across bot restarts
func (b *Bot) initializeMetrics(ctx context.Context) {
	// First, initialize expenses and categories from database (most reliable)
	if err := b.initMetricsFromDatabase(ctx); err != nil {
		b.logger.Error(ctx, "failed to initialize metrics from database", "err", err)
	}

	// Then, try to restore other metrics from Prometheus
	if b.prometheusClient != nil {
		if err := b.initMetricsFromPrometheus(ctx); err != nil {
			b.logger.Error(ctx, "failed to initialize metrics from prometheus, will retry periodically", "err", err)
			// Start periodic retry in background
			go b.retryMetricInitialization(context.Background())
			return
		}
		b.logger.Print(ctx, "metrics successfully initialized from prometheus")
		return
	}

	// Prometheus client not available, start retry in background
	b.logger.Error(ctx, "prometheus client not available, will retry initialization")
	go b.retryMetricInitialization(context.Background())
}

// initMetricsFromDatabase initializes expenses and categories counters from database
func (b *Bot) initMetricsFromDatabase(ctx context.Context) error {
	// Get total expenses count from DB
	expenses, err := b.saldo.GetAllExpenses(ctx)
	if err != nil {
		return fmt.Errorf("failed to get expenses count: %w", err)
	}

	// Get total categories count from DB
	categories, err := b.saldo.GetAllCategories(ctx)
	if err != nil {
		return fmt.Errorf("failed to get categories count: %w", err)
	}

	// Initialize counters with database values
	expensesCreated.Add(float64(len(expenses)))
	categoriesCreated.Add(float64(len(categories)))

	b.logger.Print(ctx, "metrics initialized from database",
		"expenses", len(expenses),
		"categories", len(categories))

	return nil
}

// initMetricsFromPrometheus initializes all counter metrics from Prometheus
func (b *Bot) initMetricsFromPrometheus(ctx context.Context) error {
	if b.prometheusClient == nil {
		return errors.New("prometheus client not initialized")
	}

	// Check Prometheus health first
	if err := b.prometheusClient.CheckHealth(ctx); err != nil {
		return fmt.Errorf("prometheus health check failed: %w", err)
	}

	// Query metrics snapshot
	snapshot, err := b.prometheusClient.RestoreMetrics(ctx)
	if err != nil {
		return fmt.Errorf("failed to query metrics: %w", err)
	}

	// Restore counter metrics
	for command, count := range snapshot.CommandsProcessed {
		commandsProcessed.WithLabelValues(command).Add(count)
	}

	for msgType, count := range snapshot.MessagesProcessed {
		messagesProcessed.WithLabelValues(msgType).Add(count)
	}

	for button, count := range snapshot.ButtonsPressed {
		buttonsPressed.WithLabelValues(button).Add(count)
	}

	for action, count := range snapshot.CallbacksProcessed {
		callbacksProcessed.WithLabelValues(action).Add(count)
	}

	for errType, count := range snapshot.ErrorsTotal {
		errorsTotal.WithLabelValues(errType).Add(count)
	}

	b.logger.Print(ctx, "metrics restored from prometheus",
		"commands", len(snapshot.CommandsProcessed),
		"messages", len(snapshot.MessagesProcessed),
		"buttons", len(snapshot.ButtonsPressed),
		"callbacks", len(snapshot.CallbacksProcessed),
		"errors", len(snapshot.ErrorsTotal))

	return nil
}

// retryMetricInitialization periodically retries metric initialization until success
func (b *Bot) retryMetricInitialization(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	retryCount := 0
	maxRetries := 20 // Max 10 minutes (20 * 30 seconds)

	for {
		select {
		case <-ticker.C:
			retryCount++
			b.logger.Print(ctx, "retrying metric initialization from prometheus", "attempt", retryCount)

			// Try to create client if it doesn't exist
			if b.prometheusClient == nil {
				prometheusURL := "http://prometheus:9090"
				client, err := services.NewPrometheusClient(prometheusURL, b.logger)
				if err != nil {
					b.logger.Error(ctx, "failed to create prometheus client", "err", err)
					if retryCount >= maxRetries {
						b.logger.Error(ctx, "max retries reached, giving up on metric initialization")
						return
					}
					continue
				}
				b.prometheusClient = client
			}

			// Try to initialize metrics
			if err := b.initMetricsFromPrometheus(ctx); err != nil {
				b.logger.Error(ctx, "failed to initialize metrics", "err", err, "attempt", retryCount)
				if retryCount >= maxRetries {
					b.logger.Error(ctx, "max retries reached, giving up on metric initialization")
					return
				}
				continue
			}

			// Success!
			b.logger.Print(ctx, "metrics successfully initialized from prometheus after retries", "attempts", retryCount)
			return

		case <-ctx.Done():
			b.logger.Print(ctx, "metric initialization retry cancelled")
			return
		}
	}
}

// registerHandlers registers all command handlers
func (b *Bot) registerHandlers() {
	// Command handlers
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, b.handleStart)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypeExact, b.handleHelp)

	// Callback query handler for inline keyboards
	b.api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "", bot.MatchTypePrefix, b.handleCallback)

	// Text message handler (for state-based conversations and keyboard buttons)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, b.handleMessage)
}

// defaultHandler handles unknown messages
func defaultHandler(logger embedlog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message != nil {
			logger.Print(ctx, "unknown command", "text", update.Message.Text, "from", update.Message.From.Username)
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Неизвестная команда. Используйте /help для списка доступных команд.",
			})
			if err != nil {
				logger.Error(ctx, "failed to send message", "err", err)
			}
		}
	}
}

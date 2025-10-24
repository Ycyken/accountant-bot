package telegram

import (
	"context"
	"errors"
	"fmt"

	"saldo/pkg/saldo"
	"saldo/pkg/services"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/vmkteam/embedlog"
)

type Bot struct {
	api          *bot.Bot
	logger       embedlog.Logger
	saldo        *saldo.Manager
	debug        bool
	stateManager *StateManager
	transcriber  services.Transcriber
	llm          services.LLM
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

	b := &Bot{
		api:          api,
		logger:       logger,
		saldo:        saldoService,
		debug:        cfg.Debug,
		stateManager: NewStateManager(),
		transcriber:  saldo.NewLocalWhisper(),
		llm:          saldo.NewGroq(cfg.GroqToken),
	}

	// Register command handlers
	b.registerHandlers()

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

// registerHandlers registers all command handlers
func (b *Bot) registerHandlers() {
	// Command handlers
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, b.handleStart)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypeExact, b.handleHelp)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/cancel", bot.MatchTypeExact, b.handleCancel)

	// Callback query handler for inline keyboards
	b.api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "", bot.MatchTypePrefix, b.handleCallback)

	// Text message handler (for state-based conversations and keyboard buttons)
	// This will also handle voice messages
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

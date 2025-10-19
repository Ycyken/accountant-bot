package telegram

import (
	"context"
	"errors"

	"saldo/pkg/db"

	"github.com/go-telegram/bot/models"
)

// getOrCreateUser gets user by Telegram ID or creates a new one
func (b *Bot) getOrCreateUser(ctx context.Context, tgUser *models.User) (*db.User, error) {
	if tgUser == nil {
		return nil, errors.New("telegram user is nil")
	}

	return b.saldo.GetOrCreateUserByTelegramID(
		ctx,
		tgUser.ID,
		tgUser.Username,
		tgUser.FirstName,
		tgUser.LastName,
	)
}

// getUserByTelegramID gets user by Telegram user ID
func (b *Bot) getUserByTelegramID(ctx context.Context, telegramUserID int64) (*db.User, error) {
	return b.saldo.GetUserByTelegramID(ctx, telegramUserID)
}

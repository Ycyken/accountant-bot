package telegram

import (
	"context"
	"errors"

	"github.com/go-telegram/bot/models"
)

// getOrCreateUser gets user by Telegram ID or creates a new one
func (b *Bot) getOrCreateUser(ctx context.Context, tgUser *models.User) (*User, error) {
	if tgUser == nil {
		return nil, errors.New("telegram user is nil")
	}

	saldoUser, err := b.saldo.GetOrCreateUserByTelegramID(
		ctx,
		tgUser.ID,
		tgUser.Username,
		tgUser.FirstName,
		tgUser.LastName,
	)
	if err != nil {
		return nil, err
	}

	return NewUser(saldoUser), nil
}

// getUserByTelegramID gets user by Telegram user ID
func (b *Bot) getUserByTelegramID(ctx context.Context, telegramUserID int64) (*User, error) {
	saldoUser, err := b.saldo.GetUserByTelegramID(ctx, telegramUserID)
	if err != nil {
		return nil, err
	}

	return NewUser(saldoUser), nil
}

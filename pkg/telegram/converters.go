package telegram

import "saldo/pkg/saldo"

// NewUser converts saldo.User to telegram.User
func NewUser(u *saldo.User) *User {
	if u == nil {
		return nil
	}

	var firstName, lastName string
	if u.TeleramFirstName != nil {
		firstName = *u.TeleramFirstName
	}
	if u.TelegramLastName != nil {
		lastName = *u.TelegramLastName
	}

	return &User{
		ID:        u.ID,
		Username:  u.TelegramUsername,
		FirstName: firstName,
		LastName:  lastName,
	}
}

// NewCategory converts saldo.Category to telegram.Category
func NewCategory(c *saldo.Category) *Category {
	if c == nil {
		return nil
	}

	emoji := ""
	if c.Emoji != nil {
		emoji = *c.Emoji
	}

	return &Category{
		ID:     c.ID,
		UserID: c.UserID,
		Title:  c.Title,
		Emoji:  emoji,
	}
}

// NewCategories converts slice of saldo.Category to slice of telegram.Category
func NewCategories(categories []saldo.Category) []Category {
	result := make([]Category, len(categories))
	for i, cat := range categories {
		result[i] = *NewCategory(&cat)
	}
	return result
}

// NewExpense converts saldo.Expense to telegram.Expense
// nolint:unused
func NewExpense(e *saldo.Expense) *Expense {
	if e == nil {
		return nil
	}

	var category *Category
	if e.Category != nil {
		saldoCat := saldo.NewCategory(e.Category)
		category = NewCategory(saldoCat)
	}

	return &Expense{
		ID:          e.ID,
		UserID:      e.UserID,
		CategoryID:  e.CategoryID,
		Amount:      e.Amount,
		Currency:    e.Currency,
		Description: e.Description,
		CreatedAt:   e.CreatedAt,
		Category:    category,
	}
}

// NewExpenses converts slice of saldo.Expense to slice of telegram.Expense
// nolint:unused
func NewExpenses(expenses []saldo.Expense) []Expense {
	result := make([]Expense, len(expenses))
	for i, exp := range expenses {
		result[i] = *NewExpense(&exp)
	}
	return result
}

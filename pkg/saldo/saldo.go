package saldo

import (
	"context"
	"fmt"

	"saldo/pkg/db"

	"github.com/vmkteam/embedlog"
)

type Manager struct {
	cr  db.CommonRepo
	db  db.DB
	log embedlog.Logger
}

func NewManager(dbc db.DB, log embedlog.Logger) *Manager {
	return &Manager{
		cr:  db.NewCommonRepo(dbc),
		db:  dbc,
		log: log,
	}
}

// User methods

// GetOrCreateUserByTelegramID gets user by Telegram ID or creates a new one
func (s *Manager) GetOrCreateUserByTelegramID(ctx context.Context, telegramID int64, username, firstName, lastName string) (*User, error) {
	// Try to find existing user
	search := &db.UserSearch{
		TelegramID: &telegramID,
	}

	user, err := s.cr.OneUser(ctx, search)
	if err != nil {
		return nil, fmt.Errorf("failed to search user: %w", err)
	}

	// User found
	if user != nil {
		return NewUser(user), nil
	}

	// Create new user
	newUser := &db.User{
		Login:            fmt.Sprintf("tg_%d", telegramID),
		TelegramID:       telegramID,
		TelegramUsername: username,
		TeleramFirstName: &firstName,
		TelegramLastName: &lastName,
		StatusID:         db.StatusEnabled,
	}

	user, err = s.cr.AddUser(ctx, newUser)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	s.log.Print(ctx, "new user created", "user_id", user.ID, "telegram_user_id", telegramID, "username", username)

	return NewUser(user), nil
}

// GetUserByTelegramID gets user by Telegram user ID
func (s *Manager) GetUserByTelegramID(ctx context.Context, telegramUserID int64) (*User, error) {
	search := &db.UserSearch{
		TelegramID: &telegramUserID,
	}

	user, err := s.cr.OneUser(ctx, search)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return NewUser(user), nil
}

// Category methods

// GetUserCategories returns all categories for a user
func (s *Manager) GetUserCategories(ctx context.Context, userID int) ([]Category, error) {
	categories, err := s.cr.CategoriesByFilters(ctx, &db.CategorySearch{
		UserID: &userID,
	}, db.PagerDefault, s.cr.FullCategory())
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}

	return NewCategories(categories), nil
}

// GetCategoryByID returns category by ID
func (s *Manager) GetCategoryByID(ctx context.Context, categoryID int) (*Category, error) {
	category, err := s.cr.CategoryByID(ctx, categoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	return NewCategory(category), nil
}

// CreateCategory creates a new category for a user
func (s *Manager) CreateCategory(ctx context.Context, userID int, title string, emoji *string) (*Category, error) {
	category := &db.Category{
		UserID:   userID,
		Title:    title,
		Emoji:    emoji,
		StatusID: db.StatusEnabled,
	}

	createdCategory, err := s.cr.AddCategory(ctx, category)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	s.log.Print(ctx, "category created", "category_id", createdCategory.ID, "user_id", userID, "title", title)

	return NewCategory(createdCategory), nil
}

// FindOrCreateCategoryByTitle finds category by title or creates a new one
func (s *Manager) FindOrCreateCategoryByTitle(ctx context.Context, userID int, title string) (*Category, error) {
	// Try to find existing category
	categories, err := s.cr.CategoriesByFilters(ctx, &db.CategorySearch{
		UserID: &userID,
		Title:  &title,
	}, db.PagerOne, s.cr.FullCategory())
	if err != nil {
		return nil, fmt.Errorf("failed to search category: %w", err)
	}

	if len(categories) > 0 {
		return NewCategory(&categories[0]), nil
	}

	// Create new category
	return s.CreateCategory(ctx, userID, title, nil)
}

// Expense methods

// CreateExpense creates a new expense
func (s *Manager) CreateExpense(ctx context.Context, userID int, categoryID *int, amount int, currency, description string) (*Expense, error) {
	expense := &db.Expense{
		UserID:      userID,
		CategoryID:  categoryID,
		Amount:      amount,
		Currency:    currency,
		Description: description,
		StatusID:    db.StatusEnabled,
	}

	createdExpense, err := s.cr.AddExpense(ctx, expense)
	if err != nil {
		return nil, fmt.Errorf("failed to create expense: %w", err)
	}

	s.log.Print(ctx, "expense created",
		"expense_id", createdExpense.ID,
		"user_id", userID,
		"amount", amount,
		"currency", currency,
	)

	return NewExpense(createdExpense), nil
}

// CreateExpenseWithCategory creates expense and finds/creates category if needed
func (s *Manager) CreateExpenseWithCategory(ctx context.Context, userID int, amount int, currency, categoryTitle, description string) (*Expense, error) {
	var categoryID *int

	if categoryTitle != "" {
		category, err := s.FindOrCreateCategoryByTitle(ctx, userID, categoryTitle)
		if err != nil {
			return nil, fmt.Errorf("failed to find or create category: %w", err)
		}
		categoryID = &category.ID
	}

	return s.CreateExpense(ctx, userID, categoryID, amount, currency, description)
}

// GetUserExpenses returns expenses for a user with optional filters
func (s *Manager) GetUserExpenses(ctx context.Context, userID int) ([]Expense, error) {
	expenses, err := s.cr.ExpensesByFilters(ctx, &db.ExpenseSearch{
		UserID: &userID,
	}, db.PagerDefault, s.cr.FullExpense())
	if err != nil {
		return nil, fmt.Errorf("failed to get expenses: %w", err)
	}

	return NewExpenses(expenses), nil
}

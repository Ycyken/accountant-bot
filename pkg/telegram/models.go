package telegram

import "time"

// User represents a user in the telegram bot layer
type User struct {
	ID        int
	Username  string
	FirstName string
	LastName  string
}

// Category represents an expense category in the telegram bot layer
type Category struct {
	ID     int
	UserID int
	Title  string
	Emoji  string
}

// Expense represents a user expense in the telegram bot layer
type Expense struct {
	ID          int
	UserID      int
	CategoryID  *int
	Amount      int // in cents
	Currency    string
	Description string
	CreatedAt   time.Time

	// Relations
	Category *Category
}

// CreateCategoryRequest represents a request to create a category
type CreateCategoryRequest struct {
	UserID int
	Title  string
	Emoji  string
}

// CreateExpenseRequest represents a request to create an expense
type CreateExpenseRequest struct {
	UserID         int
	Amount         int // in cents
	Currency       string
	CategoryTitle  string
	Description    string
	CreateCategory bool // auto-create category if not found
}

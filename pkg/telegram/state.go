package telegram

import (
	"sync"
)

// UserState represents the current state of a user in conversation flow
type UserState string

const (
	StateIdle                    UserState = "idle"
	StateAwaitingCategoryName    UserState = "awaiting_category_name"
	StateAwaitingCategoryEmoji   UserState = "awaiting_category_emoji"
	StateAwaitingExpense         UserState = "awaiting_expense"
	StateAwaitingExpenseCategory UserState = "awaiting_expense_category"
	StateAwaitingDescription     UserState = "awaiting_description"
)

// UserStateData holds temporary data for user's current operation
type UserStateData struct {
	State        UserState
	CategoryName string
	ExpenseData  *ExpenseData
}

// ExpenseData holds parsed expense information
type ExpenseData struct {
	Amount      int // in cents
	Currency    string
	Category    string
	Description string
}

// StateManager manages user states across conversations
type StateManager struct {
	mu     sync.RWMutex
	states map[int64]*UserStateData
}

// NewStateManager creates a new state manager
func NewStateManager() *StateManager {
	return &StateManager{
		states: make(map[int64]*UserStateData),
	}
}

// GetState returns the current state for a user
func (sm *StateManager) GetState(telegramUserID int64) *UserStateData {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if state, exists := sm.states[telegramUserID]; exists {
		return state
	}
	return &UserStateData{State: StateIdle}
}

// SetState sets the state for a user
func (sm *StateManager) SetState(telegramUserID int64, state UserState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if existing, exists := sm.states[telegramUserID]; exists {
		existing.State = state
	} else {
		sm.states[telegramUserID] = &UserStateData{State: state}
	}
}

// SetStateData sets complete state data for a user
func (sm *StateManager) SetStateData(telegramUserID int64, data *UserStateData) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.states[telegramUserID] = data
}

// ClearState clears the state for a user
func (sm *StateManager) ClearState(telegramUserID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.states, telegramUserID)
}

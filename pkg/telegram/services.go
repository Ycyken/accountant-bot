package telegram

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/vmkteam/embedlog"
)

// WhisperService handles voice transcription
type WhisperService interface {
	Transcribe(ctx context.Context, audioFilePath string) (string, error)
}

// LLMService handles expense parsing from text
type LLMService interface {
	ParseExpense(ctx context.Context, text string, userCategories []string) (*ParsedExpense, error)
}

// ParsedExpense represents parsed expense data from LLM
type ParsedExpense struct {
	Amount           float64
	Currency         string
	Category         string
	Description      string
	NeedsCategory    bool
	NeedsDescription bool
}

// MockWhisperService is a mock implementation of WhisperService
type MockWhisperService struct {
	logger embedlog.Logger
}

// NewMockWhisperService creates a new mock whisper service
func NewMockWhisperService(logger embedlog.Logger) *MockWhisperService {
	return &MockWhisperService{logger: logger}
}

// Transcribe mocks transcription of audio file
func (m *MockWhisperService) Transcribe(ctx context.Context, audioFilePath string) (string, error) {
	m.logger.Print(ctx, "mock whisper transcribe", "file", audioFilePath)

	// Mock response - in real implementation this would call whisper.cpp
	return "купил еды на 500 рублей в категории еда", nil
}

// MockLLMService is a mock implementation of LLMService
type MockLLMService struct {
	logger embedlog.Logger
}

// NewMockLLMService creates a new mock LLM service
func NewMockLLMService(logger embedlog.Logger) *MockLLMService {
	return &MockLLMService{logger: logger}
}

// ParseExpense mocks parsing of expense text using LLM
func (m *MockLLMService) ParseExpense(ctx context.Context, text string, userCategories []string) (*ParsedExpense, error) {
	m.logger.Print(ctx, "mock llm parse expense", "text", text, "categories", userCategories)

	// Simple pattern matching (mock LLM behavior)
	parsed := &ParsedExpense{
		Currency: "RUB",
	}

	// Extract amount
	amountRegex := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:рубл|руб|₽|dollars?|usd|\$|евро|eur|€)?`)
	if matches := amountRegex.FindStringSubmatch(strings.ToLower(text)); len(matches) > 1 {
		amount, _ := strconv.ParseFloat(matches[1], 64)
		parsed.Amount = amount
	} else {
		parsed.Amount = 0
	}

	// Extract currency
	textLower := strings.ToLower(text)
	if strings.Contains(textLower, "dollar") || strings.Contains(textLower, "usd") || strings.Contains(textLower, "$") {
		parsed.Currency = "USD"
	} else if strings.Contains(textLower, "euro") || strings.Contains(textLower, "eur") || strings.Contains(textLower, "€") {
		parsed.Currency = "EUR"
	}

	// Extract category
	categoryFound := false
	for _, cat := range userCategories {
		if strings.Contains(textLower, strings.ToLower(cat)) {
			parsed.Category = cat
			categoryFound = true
			break
		}
	}

	// Common category keywords
	categoryKeywords := map[string]string{
		"еда":           "Еда",
		"food":          "Еда",
		"транспорт":     "Транспорт",
		"transport":     "Транспорт",
		"дом":           "Дом",
		"home":          "Дом",
		"развлечени":    "Развлечения",
		"entertainment": "Развлечения",
		"здоровье":      "Здоровье",
		"health":        "Здоровье",
		"покупк":        "Покупки",
		"shopping":      "Покупки",
	}

	if !categoryFound {
		for keyword, category := range categoryKeywords {
			if strings.Contains(textLower, keyword) {
				parsed.Category = category
				categoryFound = true
				break
			}
		}
	}

	if !categoryFound {
		parsed.NeedsCategory = true
	}

	// Extract description (everything that's not amount or category)
	description := text
	description = amountRegex.ReplaceAllString(description, "")

	// Remove category mentions
	for _, cat := range userCategories {
		description = strings.ReplaceAll(description, cat, "")
		description = strings.ReplaceAll(description, strings.ToLower(cat), "")
	}

	// Remove common words
	wordsToRemove := []string{"купил", "потратил", "на", "в категории", "category", "spent", "bought"}
	for _, word := range wordsToRemove {
		description = strings.ReplaceAll(strings.ToLower(description), word, "")
	}

	description = strings.TrimSpace(description)

	if description == "" || len(description) < 3 {
		parsed.NeedsDescription = true
		parsed.Description = ""
	} else {
		parsed.Description = description
	}

	return parsed, nil
}

// FormatExpenseDetails formats expense details for user confirmation
func FormatExpenseDetails(expense *ParsedExpense) string {
	return fmt.Sprintf(
		"💰 <b>Сумма:</b> %.2f %s\n"+
			"📂 <b>Категория:</b> %s\n"+
			"📝 <b>Описание:</b> %s",
		expense.Amount,
		expense.Currency,
		expense.Category,
		expense.Description,
	)
}

package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/vmkteam/embedlog"
)

// LLM handles expense parsing from text
type LLM interface {
	ParseExpenses(ctx context.Context, text string, userCategories []string) ([]ParsedExpense, error)
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
func (m *MockLLMService) ParseExpenses(ctx context.Context, text string, userCategories []string) (*ParsedExpense, error) {
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
				break
			}
		}
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
		parsed.Description = ""
	} else {
		parsed.Description = description
	}

	return parsed, nil
}

// FormatExpenseDetails formats expenses details for user confirmation
func FormatExpenseDetails(expenses []ParsedExpense) string {
	var b strings.Builder

	for _, e := range expenses {
		fmt.Fprintf(&b, "💰 %.2f %s — %s", e.Amount, e.Currency, e.Category)
		if e.Description != "" {
			fmt.Fprintf(&b, " (%s)", e.Description)
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

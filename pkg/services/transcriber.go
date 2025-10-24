package services

import (
	"context"

	"github.com/vmkteam/embedlog"
)

// Transcriber handles voice transcription
type Transcriber interface {
	Transcribe(ctx context.Context, audioFilePath string) (string, error)
}

// ParsedExpense represents parsed expense data from LLM
type ParsedExpense struct {
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
}

// MockTranscriber is a mock implementation of Transcriber
type MockTranscriber struct {
	logger embedlog.Logger
}

// NewMockTranscriber creates a new mock transcriber
func NewMockTranscriber(logger embedlog.Logger) *MockTranscriber {
	return &MockTranscriber{logger: logger}
}

// Transcribe mocks transcription of audio file
func (m *MockTranscriber) Transcribe(ctx context.Context, audioFilePath string) (string, error) {
	m.logger.Print(ctx, "mock transcriber", "file", audioFilePath)

	// Mock response - in real implementation this would call whisper.cpp
	return "купил еды на 500 рублей в категории еда", nil
}

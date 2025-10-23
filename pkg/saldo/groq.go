package saldo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"saldo/pkg/services"
)

const model = "llama-3.1-8b-instant"

type Groq struct {
	token string
}

func NewGroq(token string) *Groq {
	return &Groq{
		token: token,
	}
}

type groqRequest struct {
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	Messages    []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// const prompt = `some prompt about expenses`

func (g *Groq) callGroq(ctx context.Context, userMessage string) (string, error) {
	reqBody := groqRequest{
		Model: model,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "user", Content: userMessage},
		},
	}

	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.groq.com/openai/v1/chat/completions",
		bytes.NewBuffer(jsonData))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api error: %s", string(body))
	}

	var result groqResponse
	json.Unmarshal(body, &result)

	if len(result.Choices) == 0 {
		return "", errors.New("no response from groq")
	}

	return result.Choices[0].Message.Content, nil
}

func (g *Groq) ParseExpense(ctx context.Context, text string, userCategories []string) (*services.ParsedExpense, error) {
	return &services.ParsedExpense{}, nil
}

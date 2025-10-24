package saldo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"saldo/pkg/services"
)

const systemPrompt = `Ты — парсер расходов. Извлеки информацию о расходах из текста и верни ТОЛЬКО валидный JSON массив.
В каждом запросе пользователь будет присылать список доступных категорий и текст расхода.
Если в тексте нет расходов, или они все с нулевой суммой, верни пустой JSON массив [].

Формат ответа (МАССИВ):
[
  {
    "amount": <целое число или число с плавающей точкой>,
    "currency": "RUB|USD|EUR",
    "category": "<непустая строка>",
    "description": "<строка или пусто>"
  }
]

Правила:
- amount всегда должен быть в формате с плавающей точкой (например: 500.0, 20.50)
- Если сумма целая — всё равно указывай десятичную часть .0 (например: 1200.0)
- Если сумма содержит копейки/центы — сохраняй точное значение
- Валюта по умолчанию RUB, если не указана
- Если описание неясно или повторяет сумму/категорию — оставь пустую строку "" в description
- Сумма всегда должна быть положительным числом
- Категория не должна быть пустой
- Сумма не должна быть нулевой -- в таком случае игнорируй такой расход
- Возвращай ТОЛЬКО JSON массив, без пояснений, текста или markdown

Правила сопоставления категорий:
- ПРИОРИТЕТ: сопоставь расход с одной из существующих категорий, если она подходит по смыслу
- Если НИ ОДНА существующая категория не подходит — создай новую осмысленную категорию
- Категория должна быть существительным в именительном падеже (например: "Еда", "Транспорт", "Развлечения")
- Будь точным: "Еда" для продуктов/ресторанов, "Транспорт" для такси/топлива, "Здоровье" для лекарств/врачей

Примеры:
Существующие категории: Еда, Транспорт, Дом

Ввод: "купил хлеба на 500 рублей"
Вывод: [{"amount": 500.0, "currency": "RUB", "category": "Еда", "description": "хлеб"}]

Ввод: "потратил 50 долларов на такси и 20 на кофе"
Вывод: [{"amount": 50.0, "currency": "USD", "category": "Транспорт", "description": "такси"}, {"amount": 20.0, "currency": "USD", "category": "Еда", "description": "кофе"}]

Ввод: "купил новый ноутбук за 50000"
Вывод: [{"amount": 50000.0, "currency": "RUB", "category": "Электроника", "description": "ноутбук"}]

Ввод: "1200 на коммуналку"
Вывод: [{"amount": 1200.0, "currency": "RUB", "category": "Дом", "description": "коммуналка"}]

Ввод: "Сегодня купил колбасу, сыр и оплатил такси"
Вывод: []

Ввод: "Сегодня гулял в парке"
Вывод: []`

const generalModel = "llama-3.1-8b-instant"
const sttModel = "whisper-large-v3-turbo"

type Groq struct {
	token string
}

func NewGroq(token string) *Groq {
	return &Groq{
		token: token,
	}
}

type groqChatRequest struct {
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

type GroqRole string

const (
	SystemRole    GroqRole = "system"
	UserRole      GroqRole = "user"
	AssistantRole GroqRole = "assistant"
)

func (g *Groq) callChat(ctx context.Context, userPrompt string) (string, error) {
	const endpoint = "https://api.groq.com/openai/v1/chat/completions"
	reqBody := groqChatRequest{
		Model: generalModel,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: string(SystemRole), Content: systemPrompt},
			{Role: string(UserRole), Content: userPrompt},
		},
	}

	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		endpoint,
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
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", fmt.Errorf("failed to parse groq response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", errors.New("no response from groq")
	}

	return result.Choices[0].Message.Content, nil
}

func buildExpensePrompt(text string, userCategories []string) string {
	categories := strings.Join(userCategories, ", ")
	return fmt.Sprintf("Существующие категории: %s\n\nТекст пользователя с расходами: %s\n", categories, text)
}

func (g *Groq) ParseExpenses(ctx context.Context, text string, userCategories []string) ([]services.ParsedExpense, error) {
	userPrompt := buildExpensePrompt(text, userCategories)

	response, err := g.callChat(ctx, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("groq api call failed: %w", err)
	}

	var expenses []services.ParsedExpense
	if err := json.Unmarshal([]byte(response), &expenses); err != nil {
		return nil, fmt.Errorf("failed to parse groq response: %w, response: %s", err, response)
	}

	return expenses, nil
}

func NewAudioRequest(filePath string, fields map[string]string) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	f, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, "", err
	}
	_, err = io.Copy(part, f)
	if err != nil {
		return nil, "", err
	}

	for k, v := range fields {
		err = writer.WriteField(k, v)
		if err != nil {
			return nil, "", err
		}
	}
	writer.Close()
	return body, writer.FormDataContentType(), nil
}

func (g *Groq) callTranscription(ctx context.Context, audioFilePath string) (string, error) {
	const endpoint = "https://api.groq.com/openai/v1/audio/transcriptions"

	fields := map[string]string{
		"model":       sttModel,
		"language":    "ru",
		"temperature": "0",
	}
	body, contentType, err := NewAudioRequest(audioFilePath, fields)
	if err != nil {
		return "", fmt.Errorf("build request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api error: %s", string(respBody))
	}

	var text struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &text); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	return text.Text, nil
}

func (g *Groq) Transcribe(ctx context.Context, oggFilePath string) (string, error) {
	tmpWav, err := ConvertOggToWav(ctx, oggFilePath)
	if err != nil {
		return "", fmt.Errorf("convert ogg to wav: %w", err)
	}
	defer os.Remove(tmpWav)

	text, err := g.callTranscription(ctx, tmpWav)
	if err != nil {
		return "", fmt.Errorf("transcription failed: %w", err)
	}

	return text, nil
}

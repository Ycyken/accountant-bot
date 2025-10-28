package telegram

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Telegram bot metrics
var (
	// Счетчик обработанных команд по типам
	commandsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telegram_commands_processed_total",
			Help: "Total number of processed commands by type",
		},
		[]string{"command"}, // start, help
	)

	// Счетчик обработанных сообщений по типам
	messagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telegram_messages_processed_total",
			Help: "Total number of processed messages by type",
		},
		[]string{"type"}, // text, voice
	)

	// Счетчик нажатий на кнопки по типам
	buttonsPressed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telegram_buttons_pressed_total",
			Help: "Total number of button presses by type",
		},
		[]string{"button"}, // add_expense, statistics, back, period_today, etc
	)

	// Счетчик обработанных callback запросов по действиям
	callbacksProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telegram_callbacks_processed_total",
			Help: "Total number of processed callback queries by action",
		},
		[]string{"action"}, // confirm, cancel
	)

	// Счетчик созданных расходов
	expensesCreated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "telegram_expenses_created_total",
			Help: "Total number of expenses created",
		},
	)

	// Счетчик созданных категорий
	categoriesCreated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "telegram_categories_created_total",
			Help: "Total number of categories created",
		},
	)

	// Счетчик ошибок по типам
	errorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telegram_errors_total",
			Help: "Total number of errors by type",
		},
		[]string{"type"}, // transcription, llm_parse, llm_parse_failed, database, download_file, user_not_found, get_categories
	)

	// Гистограмма времени транскрибации
	transcriptionDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "telegram_transcription_duration_seconds",
			Help:    "Duration of voice transcription in seconds",
			Buckets: []float64{0.5, 1.5, 2.5, 3.5},
		},
	)

	// Гистограмма времени парсинга LLM
	llmParseDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "telegram_llm_parse_duration_seconds",
			Help:    "Duration of LLM expense parsing in seconds",
			Buckets: []float64{0.5, 1.5, 2.5, 3.5},
		},
	)
)

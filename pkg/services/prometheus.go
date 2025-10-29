package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// MetricsSnapshot contains restored metric values
type MetricsSnapshot struct {
	CommandsProcessed  map[string]float64 // command -> count
	MessagesProcessed  map[string]float64 // type -> count
	ButtonsPressed     map[string]float64 // button -> count
	CallbacksProcessed map[string]float64 // action -> count
	ErrorsTotal        map[string]float64 // type -> count
}

// PrometheusClient wraps Prometheus API client
type PrometheusClient struct {
	api    v1.API
	logger Logger
}

// Logger interface for prometheus client
type Logger interface {
	Print(ctx context.Context, msg string, args ...interface{})
	Error(ctx context.Context, msg string, args ...interface{})
}

// NewPrometheusClient creates a new Prometheus API client
func NewPrometheusClient(prometheusURL string, logger Logger) (*PrometheusClient, error) {
	// Allow override via environment variable
	if envURL := os.Getenv("PROMETHEUS_URL"); envURL != "" {
		prometheusURL = envURL
	}

	client, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	return &PrometheusClient{
		api:    v1.NewAPI(client),
		logger: logger,
	}, nil
}

// RestoreMetrics queries Prometheus for last known metric values
func (p *PrometheusClient) RestoreMetrics(ctx context.Context) (*MetricsSnapshot, error) {
	snapshot := &MetricsSnapshot{
		CommandsProcessed:  make(map[string]float64),
		MessagesProcessed:  make(map[string]float64),
		ButtonsPressed:     make(map[string]float64),
		CallbacksProcessed: make(map[string]float64),
		ErrorsTotal:        make(map[string]float64),
	}

	// Query all counter metrics (except expenses/categories which are restored from database)
	queries := map[string]string{
		"commands":  "telegram_commands_processed_total",
		"messages":  "telegram_messages_processed_total",
		"buttons":   "telegram_buttons_pressed_total",
		"callbacks": "telegram_callbacks_processed_total",
		"errors":    "telegram_errors_total",
	}

	for name, query := range queries {
		result, warnings, err := p.api.Query(ctx, query, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to query %s: %w", name, err)
		}

		if len(warnings) > 0 {
			p.logger.Print(ctx, "prometheus query warnings", "metric", name, "warnings", warnings)
		}

		// Debug logging
		p.logger.Print(ctx, "prometheus query result", "metric", name, "type", fmt.Sprintf("%T", result), "value", result)

		// Parse result based on metric type
		switch name {
		case "commands":
			snapshot.CommandsProcessed = p.parseVectorWithLabels(result, "command")
		case "messages":
			snapshot.MessagesProcessed = p.parseVectorWithLabels(result, "type")
		case "buttons":
			snapshot.ButtonsPressed = p.parseVectorWithLabels(result, "button")
		case "callbacks":
			snapshot.CallbacksProcessed = p.parseVectorWithLabels(result, "action")
		case "errors":
			snapshot.ErrorsTotal = p.parseVectorWithLabels(result, "type")
		}
	}

	return snapshot, nil
}

// parseVectorWithLabels extracts values from vector result grouped by label
func (p *PrometheusClient) parseVectorWithLabels(value model.Value, labelName string) map[string]float64 {
	result := make(map[string]float64)

	if value == nil {
		return result
	}

	vector, ok := value.(model.Vector)
	if !ok {
		return result
	}

	for _, sample := range vector {
		labelValue := string(sample.Metric[model.LabelName(labelName)])
		result[labelValue] = float64(sample.Value)
	}

	return result
}

// CheckHealth verifies Prometheus is accessible
func (p *PrometheusClient) CheckHealth(ctx context.Context) error {
	// Try to get build info as health check
	_, err := p.api.Buildinfo(ctx)
	return err
}

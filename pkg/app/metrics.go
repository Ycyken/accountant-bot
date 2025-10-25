package app

import (
	"fmt"

	monitor "github.com/hypnoglow/go-pg-monitor"
	"github.com/hypnoglow/go-pg-monitor/gopgv10"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vmkteam/appkit"
)

// registerMetrics is a function that initializes metrics and adds /metrics endpoint to echo.
// This endpoint exposes:
// - HTTP metrics (via appkit.HTTPMetrics)
// - Database connection metrics (via go-pg-monitor)
// - Telegram bot metrics (auto-registered via promauto in pkg/telegram/metrics.go)
func (a *App) registerMetrics() {
	// add db conn metrics
	dbMetrics := monitor.NewMetrics(monitor.MetricsWithConstLabels(prometheus.Labels{"connection_name": "default"}))
	dbOpts := a.db.Options()
	a.mon = monitor.NewMonitor(
		gopgv10.NewObserver(a.db.DB),
		dbMetrics,
		monitor.MonitorWithPoolName(fmt.Sprintf("%s/%s", dbOpts.Addr, dbOpts.Database)),
	)
	a.mon.Open()

	// Add HTTP metrics middleware
	a.echo.Use(appkit.HTTPMetrics(appkit.DefaultServerName))

	// Expose all metrics via /metrics endpoint
	// Note: Telegram bot metrics are automatically registered in default Prometheus registry
	// via promauto in pkg/telegram/metrics.go
	a.echo.Any("/metrics", echo.WrapHandler(promhttp.Handler()))
}

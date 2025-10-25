package app

import (
	"context"
	"time"

	"saldo/pkg/db"
	"saldo/pkg/saldo"
	"saldo/pkg/telegram"

	"github.com/go-pg/pg/v10"
	monitor "github.com/hypnoglow/go-pg-monitor"
	"github.com/labstack/echo/v4"
	"github.com/vmkteam/appkit"
	"github.com/vmkteam/embedlog"
)

type Config struct {
	Database *pg.Options
	Server   struct {
		Host    string
		Port    int
		IsDevel bool
	}
	Telegram struct {
		Token string
		Debug bool
	}
	Sentry struct {
		Environment string
		DSN         string
	}
	Groq struct {
		Token string
	}
}

type App struct {
	embedlog.Logger
	appName string
	cfg     Config
	db      db.DB
	mon     *monitor.Monitor
	echo    *echo.Echo
	tgBot   *telegram.Bot
}

func New(ctx context.Context, appName string, sl embedlog.Logger, cfg Config, dbc db.DB) (*App, error) {
	a := &App{
		appName: appName,
		cfg:     cfg,
		db:      dbc,
		echo:    appkit.NewEcho(),
		Logger:  sl,
	}

	if cfg.Telegram.Token != "" {
		saldoService := saldo.NewManager(dbc, sl)

		tgBot, err := telegram.New(ctx, telegram.Config{
			Token:     cfg.Telegram.Token,
			Debug:     cfg.Telegram.Debug,
			GroqToken: cfg.Groq.Token,
		}, saldoService, sl)
		if err != nil {
			return nil, err
		}
		a.tgBot = tgBot
	}

	return a, nil
}

// Run is a function that runs application.
func (a *App) Run(ctx context.Context) error {
	a.registerMetrics()
	a.registerHandlers()
	a.registerDebugHandlers()
	a.registerMetadata()

	// Start Telegram bot if configured
	if a.tgBot != nil {
		go func() {
			if err := a.tgBot.Start(ctx); err != nil {
				a.Error(ctx, "telegram bot error", "err", err)
			}
		}()
	}

	return a.runHTTPServer(ctx, a.cfg.Server.Host, a.cfg.Server.Port)
}

// Shutdown is a function that gracefully stops HTTP server.
func (a *App) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Stop Telegram bot
	if a.tgBot != nil {
		a.tgBot.Stop(ctx)
	}

	a.mon.Close()

	return a.echo.Shutdown(ctx)
}

// registerMetadata is a function that registers meta info from service.
func (a *App) registerMetadata() {
	services := []appkit.ServiceMetadata{}
	if a.tgBot != nil {
		// Telegram bot runs asynchronously in a separate goroutine
		services = append(services, appkit.NewServiceMetadata("telegram-bot", appkit.MetadataServiceTypeAsync))
	}

	opts := appkit.MetadataOpts{
		HasPublicAPI:  false, // No public API, only Telegram bot
		HasPrivateAPI: false,
		DBs: []appkit.DBMetadata{
			appkit.NewDBMetadata(a.cfg.Database.Database, a.cfg.Database.PoolSize, false),
		},
		Services: services,
	}

	md := appkit.NewMetadataManager(opts)
	md.RegisterMetrics()

	a.echo.GET("/debug/metadata", md.Handler)
}

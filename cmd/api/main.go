package main

import (
	"context"
	"flag"
	"log/slog"
	"net/mail"
	"net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	"github.com/micahco/api/internal/mailer"
	"github.com/micahco/api/internal/models"
	"github.com/micahco/api/ui"
)

const version = "0.0.1"

type config struct {
	port int
	dev  bool
	db   struct {
		dsn string
	}
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
}

type application struct {
	baseURL *url.URL
	config  config
	logger  *slog.Logger
	mailer  *mailer.Mailer
	models  models.Models
}

func main() {
	var cfg config
	var urlstr string

	// Default flag values for production
	flag.IntVar(&cfg.port, "port", 8080, "API server port")
	flag.BoolVar(&cfg.dev, "dev", false, "Development mode")
	flag.StringVar(&urlstr, "url", "", "Base URL")

	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")

	flag.StringVar(&cfg.smtp.host, "smtp-host", "", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 2525, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-user", "", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-pass", "", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-addr", "", "SMTP sender address")

	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	flag.Parse()

	// Logger
	h := newSlogHandler(cfg)
	logger := slog.New(h)
	// Create error log for http.Server
	errLog := slog.NewLogLogger(h, slog.LevelError)

	// Base URL
	baseURL, err := url.Parse(urlstr)
	if err != nil {
		fatal(logger, err)
	}

	// PostgreSQL
	pool, err := openPool(cfg)
	if err != nil {
		fatal(logger, err)
	}
	defer pool.Close()

	// Mailer
	sender := &mail.Address{
		Name:    "Do Not Reply",
		Address: cfg.smtp.sender,
	}
	logger.Debug("dialing SMTP server...")
	mailer, err := mailer.New(
		cfg.smtp.host,
		cfg.smtp.port,
		cfg.smtp.username,
		cfg.smtp.password,
		sender,
		ui.Files,
		"mail/*.tmpl",
	)
	if err != nil {
		fatal(logger, err)
	}

	app := &application{
		baseURL: baseURL,
		config:  cfg,
		logger:  logger,
		mailer:  mailer,
		models:  models.New(pool),
	}

	err = app.serve(errLog)
	if err != nil {
		fatal(logger, err)
	}
}

func openPool(cfg config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbpool, err := pgxpool.New(ctx, cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	err = dbpool.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return dbpool, err
}

func newSlogHandler(cfg config) slog.Handler {
	if cfg.dev {
		// Development text hanlder
		return tint.NewHandler(os.Stdout, &tint.Options{
			AddSource:  true,
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		})
	}

	// Production use JSON handler with default opts
	return slog.NewJSONHandler(os.Stdout, nil)
}

func fatal(logger *slog.Logger, err error) {
	logger.Error("fatal", slog.Any("err", err))
	os.Exit(1)
}

func (app *application) background(fn func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				app.logger.Error("background", slog.Any("err", err))
			}
		}()
		fn()
	}()
}

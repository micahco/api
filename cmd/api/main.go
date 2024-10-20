package main

import (
	"context"
	"expvar"
	"flag"
	"log/slog"
	"net/mail"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	"github.com/micahco/api/internal/data"
	"github.com/micahco/api/internal/mailer"
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
	cors struct {
		trustedOrigins []string
	}
}

func main() {
	var cfg config

	// Default flag values for production
	flag.IntVar(&cfg.port, "port", 8080, "API server port")
	flag.BoolVar(&cfg.dev, "dev", false, "Development mode")

	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")

	flag.StringVar(&cfg.smtp.host, "smtp-host", "", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 2525, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "", "SMTP sender")

	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	flag.Parse()

	// Logger
	h := newSlogHandler(cfg)
	logger := slog.New(h)
	// Create error log for http.Server
	errLog := slog.NewLogLogger(h, slog.LevelError)

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

	expvar.NewString("version").Set(version)
	expvar.Publish("goroutines", expvar.Func(func() interface{} {
		return runtime.NumGoroutine()
	}))
	expvar.Publish("database", expvar.Func(func() interface{} {
		return dbStats(pool.Stat())
	}))

	app := &application{
		config: cfg,
		logger: logger,
		mailer: mailer,
		models: data.New(pool),
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

type poolStats struct {
	AcquireCount            int64
	AcquireDuration         time.Duration
	AcquiredConns           int32
	CanceledAcquireCount    int64
	ConstructingConns       int32
	EmptyAcquireCount       int64
	IdleConns               int32
	MaxConns                int32
	MaxIdleDestroyCount     int64
	MaxLifetimeDestroyCount int64
	NewConnsCount           int64
	TotalConns              int32
}

func dbStats(st *pgxpool.Stat) poolStats {
	return poolStats{
		AcquireCount:            st.AcquireCount(),
		AcquireDuration:         st.AcquireDuration(),
		AcquiredConns:           st.AcquiredConns(),
		CanceledAcquireCount:    st.CanceledAcquireCount(),
		ConstructingConns:       st.ConstructingConns(),
		EmptyAcquireCount:       st.EmptyAcquireCount(),
		IdleConns:               st.IdleConns(),
		MaxConns:                st.MaxConns(),
		MaxIdleDestroyCount:     st.MaxIdleDestroyCount(),
		MaxLifetimeDestroyCount: st.MaxLifetimeDestroyCount(),
		NewConnsCount:           st.NewConnsCount(),
		TotalConns:              st.TotalConns(),
	}
}

func fatal(logger *slog.Logger, err error) {
	logger.Error("fatal", slog.Any("err", err))
	os.Exit(1)
}

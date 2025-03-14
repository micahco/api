package main

import (
	"context"
	"expvar"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/mail"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	pgxuuid "github.com/jackc/pgx-gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	"github.com/micahco/api/internal/data"
	"github.com/micahco/api/internal/mailer"
	"github.com/micahco/api/ui"
)

var (
	buildTime string
	version   string
	logger    *slog.Logger
)

type config struct {
	port int
	dev  bool
	db   struct {
		dsn string
	}
	limiter struct {
		rps     int
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

	flag.BoolVar(&cfg.dev, "dev", false, "Development mode")
	flag.IntVar(&cfg.port, "port", getEnvInt("API_PORT"), "API server port")

	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("DATABASE_URL"), "PostgreSQL DSN")

	flag.StringVar(&cfg.smtp.host, "smtp-host", os.Getenv("API_SMTP_HOST"), "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", getEnvInt("API_SMTP_PORT"), "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", os.Getenv("API_SMTP_USERNAME"), "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", os.Getenv("API_SMTP_PASSWORD"), "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", os.Getenv("API_SMTP_SENDER"), "SMTP sender")

	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", getEnvBool("API_LIMITER_ENABLED"), "Enable rate limiter")
	flag.IntVar(&cfg.limiter.rps, "limiter-rps", getEnvInt("API_LIMITER_RPS"), "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", getEnvInt("API_LIMITER_BURST"), "Rate limiter maximum burst")

	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		if val == "" {
			val = os.Getenv("API_CORS_TRUSTED_ORIGINS")
		}

		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	displayVersion := flag.Bool("version", false, "Display version and exit")

	flag.Parse()

	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		fmt.Printf("Build time:\t%s\n", buildTime)
		os.Exit(0)
	}

	// Logger
	h := newSlogHandler(cfg.dev)
	logger = slog.New(h)
	// Create error log for http.Server
	errLog := slog.NewLogLogger(h, slog.LevelError)
	if logger == nil {
		log.Fatal("ded")
	}

	// PostgreSQL
	pool, err := openPool(cfg.db.dsn)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()

	// Mailer
	sender := &mail.Address{
		Name:    "Do Not Reply",
		Address: cfg.smtp.sender,
	}
	logger.Info("dialing SMTP server...")
	m, err := mailer.New(
		cfg.smtp.host,
		cfg.smtp.port,
		cfg.smtp.username,
		cfg.smtp.password,
		sender,
		ui.Files,
		"mail/*.tmpl",
	)
	if err != nil {
		fatal(err)
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
		mailer: m,
		models: data.New(pool),
	}

	err = app.serve(errLog)
	if err != nil {
		fatal(err)
	}
}

func openPool(dsn string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		pgxuuid.Register(conn.TypeMap())
		return nil
	}

	dbpool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	err = dbpool.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return dbpool, err
}

func newSlogHandler(dev bool) slog.Handler {
	if dev {
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

func fatal(err error) {
	if logger == nil {
		log.Fatalf("ded: %v", err)
	}

	logger.Error("fatal", slog.Any("err", err))
	os.Exit(1)
}

func getEnvInt(key string) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		return 0
	}

	v, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("getEnvInt(%v): %v", key, err)
	}

	return v
}

func getEnvBool(key string) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return false
	}

	return strings.ToLower(val) == "true"
}

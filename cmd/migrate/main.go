package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	pgxuuid "github.com/jackc/pgx-gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/micahco/api/migrations"
	"github.com/pressly/goose/v3"
)

type config struct {
	dsn   string
	up    bool
	reset bool
}

func main() {
	var cfg config

	flag.StringVar(&cfg.dsn, "dsn", os.Getenv("DATABASE_URL"), "PostgreSQL DSN")
	flag.BoolVar(&cfg.up, "up", false, "apply all up database migrations")
	flag.BoolVar(&cfg.reset, "reset", false, "reset the entire databse schema")
	flag.Parse()

	pool, err := openPool(cfg.dsn)
	if err != nil {
		log.Fatalf("pool: %s\n", err)
	}
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	goose.SetBaseFS(migrations.Files)

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("failed to set dialect: %s\n", err)
	}

	if cfg.up {
		err = migrations.Up(db)
		if err != nil {
			log.Fatalf("up: %s\n", err)
		}
	} else if cfg.reset {
		err = migrations.Reset(db)
		if err != nil {
			log.Fatalf("reset: %s\n", err)
		}
	} else {
		fmt.Println("nothing happended")
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

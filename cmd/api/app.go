package main

import (
	"log/slog"
	"net/url"
	"sync"

	"github.com/micahco/api/internal/data"
	"github.com/micahco/api/internal/mailer"
)

type application struct {
	baseURL *url.URL
	config  config
	logger  *slog.Logger
	mailer  *mailer.Mailer
	models  data.Models
	wg      sync.WaitGroup
}

func (app *application) background(fn func() error) {
	app.wg.Add(1)

	go func() {
		defer app.wg.Done()

		defer func() {
			if err := recover(); err != nil {
				app.logger.Error("background process recovered from panic", slog.Any("err", err))
			}
		}()

		if err := fn(); err != nil {
			app.logger.Error("background process returned error", slog.Any("err", err))
		}
	}()
}

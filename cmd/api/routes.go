package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// App router
func (app *application) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(app.recovery)

	r.Route("/v1", func(r chi.Router) {
		r.Get("/healthcheck", app.handle(app.handleHealthcheck))
	})

	return r
}

package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// App router
func (app *application) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(app.recovery)

	r.NotFound(app.handle(app.handleNotFound))
	r.MethodNotAllowed(app.handle(app.handleMethodNotAllowed))

	r.Route("/v1", func(r chi.Router) {
		r.Get("/healthcheck", app.handle(app.handleHealthcheck))
	})

	return r
}

func (app *application) handleNotFound(w http.ResponseWriter, r *http.Request) error {
	return app.writeJSON(
		w,
		http.StatusNotFound,
		http.StatusText(http.StatusNotFound),
		nil,
	)
}

func (app *application) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) error {
	return app.writeJSON(
		w,
		http.StatusMethodNotAllowed,
		http.StatusText(http.StatusMethodNotAllowed),
		nil,
	)
}

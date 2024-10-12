package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// App router
func (app *application) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(app.recovery)
	r.Use(app.rateLimit)

	r.NotFound(app.handle(app.handleNotFound))
	r.MethodNotAllowed(app.handle(app.handleMethodNotAllowed))

	r.Route("/v1", func(r chi.Router) {
		r.Get("/healthcheck", app.handle(app.handleHealthcheck))
	})

	return r
}

func (app *application) handleNotFound(w http.ResponseWriter, r *http.Request) error {
	return respErr{nil, http.StatusNotFound, http.StatusText(http.StatusNotFound)}
}

func (app *application) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) error {
	return respErr{nil, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)}
}

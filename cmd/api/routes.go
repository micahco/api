package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// App router
func (app *application) routes() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.StripSlashes)
	r.Use(app.metrics)
	r.Use(app.recovery)
	r.Use(app.enableCORS)
	r.Use(app.rateLimit)
	r.Use(app.authenticate)
	r.NotFound(app.handle(app.notFound))
	r.MethodNotAllowed(app.handle(app.methodNotAllowed))

	// Metrics
	r.Mount("/debug", middleware.Profiler())

	// API
	r.Route("/v1", func(r chi.Router) {
		r.Get("/healthcheck", app.handle(app.healthcheck))

		r.Route("/tokens", func(r chi.Router) {
			r.Post("/verification", app.handle(app.tokensVerificaitonPost))
			r.Post("/authentication", app.handle(app.tokensAuthenticationPost))
		})

		r.Route("/users", func(r chi.Router) {
			r.Post("/", app.handle(app.usersPost))

			r.Route("/me", func(r chi.Router) {
				r.Use(app.requireAuthentication)
				r.Get("/", app.handle(app.usersMeGet))
			})
		})
	})

	return r
}

func (app *application) notFound(w http.ResponseWriter, r *http.Request) error {
	return app.writeError(w, http.StatusNotFound, nil)
}

func (app *application) methodNotAllowed(w http.ResponseWriter, r *http.Request) error {
	return app.writeError(w, http.StatusMethodNotAllowed, nil)
}

func (app *application) healthcheck(w http.ResponseWriter, r *http.Request) error {
	env := "production"
	if app.config.dev {
		env = "development"
	}

	data := envelope{
		"status": "available",
		"system_info": map[string]string{
			"environment": env,
			"version":     version,
		},
	}

	return app.writeJSON(w, http.StatusOK, data, nil)
}

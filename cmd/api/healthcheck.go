package main

import "net/http"

func (app *application) handleHealthcheck(w http.ResponseWriter, r *http.Request) error {
	env := "production"
	if app.config.dev {
		env = "development"
	}

	data := map[string]string{
		"status":      "available",
		"environment": env,
		"version":     version,
	}

	return app.writeJSON(w, http.StatusOK, data, nil)
}

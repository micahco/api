package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type envelope map[string]interface{}

func (app *application) writeJSON(w http.ResponseWriter, statusCode int, data envelope, headers http.Header) error {
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	js = append(js, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(js)

	return nil
}

func (app *application) errorResponse(w http.ResponseWriter, statusCode int, message interface{}) {
	data := envelope{"error": message}

	err := app.writeJSON(w, statusCode, data, nil)
	if err != nil {
		app.logger.Error("unable to write error response", slog.Any("err", err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (app *application) serverErrorResponse(w http.ResponseWriter, logMsg string, err error) {
	app.logger.Error(logMsg, slog.Any("err", err))

	app.errorResponse(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
}

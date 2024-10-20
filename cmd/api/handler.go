package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation"
)

const (
	InvalidCredentailsMessage         = "invalid credentials"
	InvalidAuthenticationTokenMessage = "invalid or missing authentication token"
	AuthenticationRequiredMessage     = "you must be authenticated to access this resource"
	RateLimitExceededMessage          = "rate limit exceeded"
)

type envelope map[string]any

type withError func(w http.ResponseWriter, r *http.Request) error

// Wraps handleWithError as http.HandlerFunc, with error handling
func (app *application) handle(h withError) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			var validationError validation.Errors
			switch {
			case errors.As(err, &validationError):
				app.errorResponse(w, http.StatusUnprocessableEntity, validationError)
			default:
				app.serverErrorResponse(w, "handled unexpected error", err)
			}
		}
	}
}
func (app *application) readJSON(r *http.Request, dst any) error {
	err := json.NewDecoder(r.Body).Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return err
		}
	}

	return nil
}

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

func (app *application) writeError(w http.ResponseWriter, statusCode int, message any) error {
	if message == nil {
		message = http.StatusText(statusCode)
	}

	data := envelope{"error": message}

	return app.writeJSON(w, statusCode, data, nil)
}

func (app *application) errorResponse(w http.ResponseWriter, statusCode int, message any) {
	if message == nil {
		message = http.StatusText(statusCode)
	}

	data := envelope{"error": message}

	err := app.writeJSON(w, statusCode, data, nil)
	if err != nil {
		app.logger.Error("unable to write error response", slog.Any("err", err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (app *application) serverErrorResponse(w http.ResponseWriter, logMsg string, err error) {
	app.logger.Error(logMsg, slog.Any("err", err), slog.String("type", fmt.Sprintf("%T", err)))

	app.errorResponse(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
}

func (app *application) invalidAuthenticationTokenResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")

	app.errorResponse(w, http.StatusUnauthorized, InvalidAuthenticationTokenMessage)
}

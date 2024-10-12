package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

type withError func(w http.ResponseWriter, r *http.Request) error

// Wraps handleWithError as http.HandlerFunc, with error handling
func (app *application) handle(h withError) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			// First, check if error is a response error. A response error
			// will be intentionally created from within a withError handler.
			// This error may have a list of errors wrapped within it.
			// Only the wrapped errors will be logged.
			var respErr respErr
			if errors.As(err, &respErr) {
				// Log wrapped error(s) if exists
				if err := errors.Unwrap(respErr); err != nil {
					app.logger.Error(
						"handled unwrapped error",
						slog.Any("err", err),
					)
				}

				app.errorResponse(w, respErr.StatusCode(), respErr.Message())

				return
			}

			// Else, respond with internal server error
			app.serverErrorResponse(w, "handled unexpected error", err)
		}
	}
}

type respErr struct {
	// Wrapped error
	err error

	// Some http.StatusCode
	statusCode int

	// Message is user facing and therefore should only
	// contain information relevant to the user.
	message string
}

func (e respErr) Error() string {
	return fmt.Sprintf("respErr [%d]: %s", e.statusCode, e.message)
}

func (e respErr) Unwrap() error {
	return e.err
}

func (e respErr) StatusCode() int {
	return e.statusCode
}

// Return message if exists or generic http.StatusText
func (e respErr) Message() string {
	if e.message != "" {
		return e.message
	}

	return http.StatusText(e.statusCode)
}

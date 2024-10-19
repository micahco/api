package main

import (
	"net/http"

	"github.com/micahco/api/internal/models"
)

// Create a verification token and mail it to the user's email.
func (app *application) tokensVerificaitonPost(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Email string `json:"email"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		return err
	}

	// This will be the consistent message. Even if a user
	// already exists with this email, send this message.
	msg := envelope{"message": "A verification email has been sent. Please check your inbox."}

	// Check if user with email already exists
	exists, err := app.models.User.Exists(input.Email)
	if err != nil {
		return err
	}
	if exists {
		// User with email already exists. Send the
		// consistent respone message.
		return app.writeJSON(w, http.StatusOK, msg, nil)
	}

	// Check if a verification token has already been created recently
	exists, err = app.models.VerificationToken.Exists(input.Email)
	if err != nil {
		return err
	}
	if exists {
		// Recent verification sent, don't mail another.
		// Send the same message.
		return app.writeJSON(w, http.StatusOK, msg, nil)
	}

	t, err := app.models.VerificationToken.New(input.Email)
	if err != nil {
		return err
	}

	// Mail the plaintext token to the user's email address.
	app.background(func() error {
		data := map[string]any{
			"token": t.Plaintext,
		}

		err = app.mailer.Send(input.Email, "email-verification.tmpl", data)
		if err != nil {
			return err
		}

		return nil
	})

	return app.writeJSON(w, http.StatusOK, msg, nil)
}

func (app *application) tokensAuthenticationPost(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		return err
	}

	userID, err := app.models.User.Authenticate(input.Email, input.Password)
	if err != nil {
		if err == models.ErrInvalidCredentials {
			return app.writeError(w, http.StatusUnauthorized, InvalidCredentailsMessage)
		}

		return err
	}

	t, err := app.models.AuthenticationToken.New(int64(userID))
	if err != nil {
		return err
	}

	return app.writeJSON(w, http.StatusCreated, envelope{"authentication_token": t}, nil)
}

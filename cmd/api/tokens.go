package main

import (
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/micahco/api/internal/data"
)

const (
	verificationMsg  = "A verification email has been sent. Please check your inbox."
	passwordResetMsg = "If that email address is in our database, a token to reset your password will be sent to that address."
)

// Create a verification token with registration scope and
// mail it to the provided email address.
func (app *application) tokensVerificaitonRegistrationPost(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Email string `json:"email"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		return err
	}

	err = validation.ValidateStruct(&input,
		validation.Field(&input.Email, validation.Required, is.Email),
	)
	if err != nil {
		return err
	}

	// This will be the consistent message. Even if a user
	// already exists with this email, send this message.
	msg := envelope{"message": verificationMsg}

	// Check if user with email already exists
	exists, err := app.models.User.ExistsWithEmail(input.Email)
	if err != nil {
		return err
	}
	if exists {
		// User with email already exists. Send the
		// consistent respone message.
		return app.writeJSON(w, http.StatusOK, msg, nil)
	}

	// Check if a verification token has already been created recently
	exists, err = app.models.VerificationToken.Exists(data.ScopeRegistration, input.Email, nil)
	if err != nil {
		return err
	}
	if exists {
		// Recent verification sent, don't mail another.
		// Send the same message.
		return app.writeJSON(w, http.StatusOK, msg, nil)
	}

	t, err := app.models.VerificationToken.New(data.ScopeRegistration, input.Email, nil)
	if err != nil {
		return err
	}

	// Mail the plaintext token to the user's email address.
	app.background(func() error {
		data := map[string]any{
			"token": t.Plaintext,
		}

		return app.sendMail(input.Email, "registration.tmpl", data)
	})

	return app.writeJSON(w, http.StatusOK, msg, nil)
}

func (app *application) tokensVerificaitonEmailChangePost(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Email string `json:"email"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		return err
	}

	err = validation.ValidateStruct(&input,
		validation.Field(&input.Email, validation.Required, is.Email),
	)
	if err != nil {
		return err
	}

	// This will be the consistent message. Even if a user
	// already exists with this email, send this message.
	msg := envelope{"message": verificationMsg}

	// Check if user with email already exists
	exists, err := app.models.User.ExistsWithEmail(input.Email)
	if err != nil {
		return err
	}
	if exists {
		// User with email already exists. Send the
		// consistent respone message.
		return app.writeJSON(w, http.StatusOK, msg, nil)
	}

	// Get authenticated user's ID from context
	user := app.contextGetUser(r)

	// Check if a verification token has already been created recently
	exists, err = app.models.VerificationToken.Exists(data.ScopeEmailChange, input.Email, &user.ID)
	if err != nil {
		return err
	}
	if exists {
		// Recent verification sent, don't mail another, just
		// send the same message
		return app.writeJSON(w, http.StatusOK, msg, nil)
	}

	// Create verification token for user with new email address
	t, err := app.models.VerificationToken.New(data.ScopeEmailChange, input.Email, &user.ID)
	if err != nil {
		return err
	}

	// Mail the plaintext token to the new email address
	app.background(func() error {
		data := map[string]any{
			"token": t.Plaintext,
		}

		return app.sendMail(input.Email, "email-change.tmpl", data)
	})

	return app.writeJSON(w, http.StatusOK, msg, nil)
}

func (app *application) tokensVerificaitonPasswordResetPost(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Email string `json:"email"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		return err
	}

	err = validation.ValidateStruct(&input,
		validation.Field(&input.Email, validation.Required, is.Email),
	)
	if err != nil {
		return err
	}

	// This will be the consistent message. Even if a user
	// already exists with this email, send this message.
	msg := envelope{"message": verificationMsg}

	// Check if user with email exists
	exists, err := app.models.User.ExistsWithEmail(input.Email)
	if err != nil {
		return err
	}
	if !exists {
		// User with email does not exist. Still send the same
		// message.
		return app.writeJSON(w, http.StatusOK, msg, nil)
	}

	// Check if a verification token has already been created recently
	exists, err = app.models.VerificationToken.Exists(data.ScopePasswordReset, input.Email, nil)
	if err != nil {
		return err
	}
	if exists {
		// Recent verification sent, don't mail another
		return app.writeJSON(w, http.StatusOK, msg, nil)
	}

	t, err := app.models.VerificationToken.New(data.ScopePasswordReset, input.Email, nil)
	if err != nil {
		return err
	}

	// Mail the plaintext token to the user's email address
	app.background(func() error {
		data := map[string]any{
			"token": t.Plaintext,
		}

		return app.sendMail(input.Email, "password-reset.tmpl", data)
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

	err = validation.ValidateStruct(&input,
		validation.Field(&input.Email, validation.Required, is.Email),
		validation.Field(&input.Password, validation.Required, data.PasswordLength),
	)
	if err != nil {
		return err
	}

	user, err := app.models.User.GetForCredentials(input.Email, input.Password)
	if err != nil {
		if err == data.ErrInvalidCredentials {
			return app.writeError(w, http.StatusUnauthorized, InvalidCredentailsMessage)
		}

		return err
	}

	t, err := app.models.AuthenticationToken.New(user.ID)
	if err != nil {
		return err
	}

	return app.writeJSON(w, http.StatusCreated, envelope{"authentication_token": t}, nil)
}

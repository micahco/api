package main

import (
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/micahco/api/internal/data"
)

// Create new user with email and password if provided token
// matches verification.
func (app *application) usersPost(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Token    string `json:"token"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		return err
	}

	err = validation.ValidateStruct(&input,
		validation.Field(&input.Email, validation.Required, is.Email),
		validation.Field(&input.Password, validation.Required, data.PasswordLength),
		validation.Field(&input.Token, validation.Required),
	)
	if err != nil {
		return err
	}

	err = app.models.VerificationToken.Verify(input.Token, data.ScopeRegistration, input.Email, nil)
	if err != nil {
		switch err {
		case data.ErrRecordNotFound:
			return app.writeError(w, http.StatusUnauthorized, nil)
		case data.ErrExpiredToken:
			return app.writeError(w, http.StatusUnauthorized, "Expired token. Please signup again.")
		default:
			return err
		}
	}

	err = app.models.VerificationToken.PurgeWithEmail(input.Email)
	if err != nil {
		return err
	}

	user, err := app.models.User.New(input.Email, input.Password)
	if err != nil {
		return err
	}

	return app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
}

func (app *application) usersPasswordPut(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Password string `json:"password"`
		Token    string `json:"token"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		return err
	}

	err = validation.ValidateStruct(&input,
		validation.Field(&input.Password, validation.Required, data.PasswordLength),
		validation.Field(&input.Token, validation.Required),
	)
	if err != nil {
		return err
	}

	user, err := app.models.User.GetForVerificationToken(data.ScopePasswordReset, input.Token)
	if err != nil {
		switch {
		default:
			return err
		}
	}

	err = user.SetPasswordHash(input.Password)
	if err != nil {
		return err
	}

	err = app.models.User.Update(user)
	if err != nil {
		switch {
		default:
			return err
		}
	}

	err = app.models.VerificationToken.PurgeWithUserID(user.ID)
	if err != nil {
		return err
	}

	msg := envelope{"message": "your password was successfully reset"}

	return app.writeJSON(w, http.StatusOK, msg, nil)
}

func (app *application) usersMeGet(w http.ResponseWriter, r *http.Request) error {
	user := app.contextGetUser(r)

	return app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
}

func (app *application) usersMePut(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Email    *string `json:"email"`
		Password *string `json:"password"`
		Token    *string `json:"token"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		return err
	}

	err = validation.ValidateStruct(&input,
		validation.Field(&input.Email, is.Email),
		validation.Field(&input.Password, data.PasswordLength),
		validation.Field(&input.Token),
	)
	if err != nil {
		return err
	}

	user := app.contextGetUser(r)

	if input.Email != nil && input.Token != nil {
		err = app.models.VerificationToken.Verify(*input.Token, data.ScopeEmailChange, *input.Email, &user.ID)
		if err != nil {
			switch err {
			case data.ErrRecordNotFound:
				return app.writeError(w, http.StatusUnauthorized, nil)
			case data.ErrExpiredToken:
				return app.writeError(w, http.StatusUnauthorized, "Expired token")
			default:
				return err
			}
		}

		err = app.models.VerificationToken.PurgeWithUserID(user.ID)
		if err != nil {
			return err
		}

		user.Email = *input.Email
	}

	if input.Password != nil {
		err = user.SetPasswordHash(*input.Password)
		if err != nil {
			return err
		}
	}

	err = app.models.User.Update(user)
	if err != nil {
		return err
	}

	return app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
}

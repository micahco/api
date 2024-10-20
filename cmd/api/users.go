package main

import (
	"net/http"

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

	err = app.models.VerificationToken.Verify(input.Email, input.Token)
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

	err = app.models.VerificationToken.Purge(input.Email)
	if err != nil {
		return err
	}

	user, err := app.models.User.New(input.Email, input.Password)
	if err != nil {
		return err
	}

	return app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
}

func (app *application) usersMeGet(w http.ResponseWriter, r *http.Request) error {
	user := app.contextGetUser(r)

	return app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
}

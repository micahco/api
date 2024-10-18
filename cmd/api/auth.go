package main

import "net/http"

func (app *application) handleAuthSignup(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Email string `json:"email"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		return err
	}

	v, err := app.models.Verification.New(input.Email)
	if err != nil {
		return err
	}

	app.background(func() error {
		data := map[string]interface{}{
			"email": v.Email,
			"token": v.Plaintext,
		}

		err = app.mailer.Send(v.Email, "email-verification.tmpl", data)
		if err != nil {
			return err
		}

		return nil
	})

	msg := envelope{"message": "A verification email has been sent. Please check your inbox."}

	return app.writeJSON(w, http.StatusOK, msg, nil)
}

func (app *application) handleAuthRegister(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Token    string `json:"token"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		return err
	}

	err = app.models.Verification.Verify(input.Email, input.Token)
	if err != nil {
		return err
	}

	msg := envelope{"message": "Created user"}

	return app.writeJSON(w, http.StatusOK, msg, nil)
}

package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/micahco/web/internal/models"
	"github.com/micahco/web/internal/validator"
)

type contextKey string

const (
	authenticatedUserIDSessionKey = "authenticatedUserID"
	verificationEmailSessionKey   = "verificationEmail"
	verificationTokenSessionKey   = "verificationToken"
	resetEmailSessionKey          = "resetEmail"
	resetTokenSessionKey          = "resetToken"
	isAuthenticatedContextKey     = contextKey("isAuthenticated")
)

func (app *application) login(r *http.Request, userID int) error {
	err := app.sessionManager.RenewToken(r.Context())
	if err != nil {
		return err
	}

	app.sessionManager.Put(r.Context(), authenticatedUserIDSessionKey, userID)

	return nil
}

func (app *application) logout(r *http.Request) error {
	err := app.sessionManager.RenewToken(r.Context())
	if err != nil {
		return err
	}

	app.sessionManager.Remove(r.Context(), authenticatedUserIDSessionKey)

	return nil
}

// Checks the auth context set by the authenticate middleware
func (app *application) isAuthenticated(r *http.Request) bool {
	isAuthenticated, ok := r.Context().Value(isAuthenticatedContextKey).(bool)
	if !ok {
		return false
	}

	return isAuthenticated
}

func (app *application) getSessionUserID(r *http.Request) (int, error) {
	id, ok := app.sessionManager.Get(r.Context(), authenticatedUserIDSessionKey).(int)
	if !ok {
		return 0, fmt.Errorf("unable to parse session id as int")
	}

	return id, nil
}

type authLoginForm struct {
	email    string
	password string
	validator.Validator
}

func (app *application) handleAuthLoginPost(w http.ResponseWriter, r *http.Request) error {
	if app.isAuthenticated(r) {
		return respErr{
			statusCode: http.StatusBadRequest,
			message:    "already authenticated",
		}
	}

	err := r.ParseForm()
	if err != nil {
		return respErr{
			statusCode: http.StatusBadRequest,
			message:    "unable to parse form",
		}
	}

	form := authLoginForm{
		email:    r.Form.Get("email"),
		password: r.Form.Get("password"),
	}

	form.Validate(validator.NotBlank(form.email), "invalid email: cannot be blank")
	form.Validate(validator.Matches(form.email, validator.EmailRX), "invalid email: must be a valid email address")
	form.Validate(validator.NotBlank(form.password), "invalid password: cannot be blank")

	if !form.IsValid() {
		return validationError(form.Validator)
	}

	id, err := app.models.User.Authenticate(form.email, form.password)
	if err != nil {
		if errors.Is(err, models.ErrInvalidCredentials) {
			return respErr{statusCode: http.StatusUnauthorized}
		}

		return err
	}

	err = app.login(r, id)
	if err != nil {
		return err
	}

	// Redirect to homepage after authenticating the user.
	http.Redirect(w, r, "/", http.StatusSeeOther)

	return nil
}

func (app *application) handleAuthLogoutPost(w http.ResponseWriter, r *http.Request) error {
	err := app.logout(r)
	if err != nil {
		return err
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)

	return nil
}

type authSignupForm struct {
	email string
	validator.Validator
}

func (app *application) handleAuthSignupPost(w http.ResponseWriter, r *http.Request) error {
	if app.isAuthenticated(r) {
		return respErr{
			statusCode: http.StatusBadRequest,
			message:    "already authenticated",
		}
	}

	err := r.ParseForm()
	if err != nil {
		return respErr{
			statusCode: http.StatusBadRequest,
			message:    "unable to parse form",
		}
	}

	form := authSignupForm{email: r.Form.Get("email")}

	form.Validate(validator.NotBlank(form.email), "invalid email: cannot be blank")
	form.Validate(validator.Matches(form.email, validator.EmailRX), "invalid email: must be a valid email address")
	form.Validate(validator.MaxChars(form.email, 254), "invalid email: must be no more than 254 characters long")

	if !form.IsValid() {
		return validationError(form.Validator)
	}

	// Consistent flash message
	f := FlashMessage{
		Type:    FlashInfo,
		Message: "A link to activate your account has been sent to the email address provided. Please check your junk folder.",
	}

	// Check if user with email already exists
	exists, err := app.models.User.ExistsEmail(form.email)
	if err != nil {
		return err
	}

	// If user does exist, do nothing but send flash message
	if exists {
		app.flash(r, f)
		app.refresh(w, r)

		return nil
	}

	// Check if link verification has already been created
	v, err := app.models.Verification.Get(form.email)
	if err != nil && err != models.ErrNoRecord {
		return err
	}

	// Don't send a new link if less than 5 minutes since last
	if v != nil {
		if time.Since(v.CreatedAt) < 5*time.Minute {
			app.flash(r, f)
			app.refresh(w, r)

			return nil
		}
	}

	token, err := app.models.Verification.New(form.email)
	if err != nil {
		return fmt.Errorf("signup create token: %w", err)
	}

	// Create link with token
	ref, err := url.Parse("/auth/register")
	if err != nil {
		return err
	}
	q := ref.Query()
	q.Set("token", token)
	ref.RawQuery = q.Encode()
	link := app.baseURL.ResolveReference(ref)

	// Send mail in background routine
	if !app.config.dev {
		app.background(func() {
			err = app.mailer.Send(form.email, "email-verification.tmpl", link)
			if err != nil {
				app.logger.Error("mailer", slog.Any("err", err))
			}
		})
	}
	app.logger.Debug("mailed", slog.String("link", link.String()))

	// Clear all session data and add form email to session. That way,
	// when the user goes to register, won't have to re-enter email.
	app.sessionManager.Clear(r.Context())
	app.sessionManager.RenewToken(r.Context())
	app.sessionManager.Put(r.Context(), verificationEmailSessionKey, form.email)

	app.flash(r, f)
	app.refresh(w, r)

	return nil
}

type authRegisterData struct {
	HasSessionEmail bool
}

func (app *application) handleAuthRegisterGet(w http.ResponseWriter, r *http.Request) error {
	if app.isAuthenticated(r) {
		return respErr{
			statusCode: http.StatusBadRequest,
			message:    "already authenticated",
		}
	}

	queryToken := r.URL.Query().Get("token")
	if queryToken == "" {
		return respErr{
			statusCode: http.StatusBadRequest,
			message:    "missing verification token",
		}
	}

	app.sessionManager.Put(r.Context(), verificationTokenSessionKey, queryToken)

	// If session email exists, don't show email input in form.
	hasSessionEmail := app.sessionManager.Exists(r.Context(), verificationEmailSessionKey)
	data := authRegisterData{
		HasSessionEmail: hasSessionEmail,
	}

	return app.render(w, r, http.StatusOK, "auth-register.tmpl", data)
}

type authRegisterForm struct {
	email    string
	password string
	validator.Validator
}

var ExpiredTokenFlash = FlashMessage{
	Type:    FlashError,
	Message: "Expired verification token.",
}

func (app *application) handleAuthRegisterPost(w http.ResponseWriter, r *http.Request) error {
	if app.isAuthenticated(r) {
		return respErr{
			statusCode: http.StatusBadRequest,
			message:    "already authenticated",
		}
	}

	err := r.ParseForm()
	if err != nil {
		return respErr{
			statusCode: http.StatusBadRequest,
			message:    "unable to parse form",
		}
	}

	form := authRegisterForm{
		email:    r.Form.Get("email"),
		password: r.Form.Get("password"),
	}

	email := app.sessionManager.GetString(r.Context(), verificationEmailSessionKey)
	if form.email != "" {
		email = form.email
	}

	form.Validate(validator.NotBlank(email), "invalid login email: cannot be blank")
	form.Validate(validator.Matches(email, validator.EmailRX), "invalid login email: must be a valid email address")
	form.Validate(validator.MaxChars(email, 254), "invalid login email: must be no more than 254 characters long")
	form.Validate(validator.NotBlank(form.password), "invalid password: cannot be blank")
	form.Validate(validator.MinChars(form.password, 8), "invalid password: must be at least 8 characters long")
	form.Validate(validator.MaxChars(form.password, 72), "invalid password: must be no more than 72 characters long")

	if !form.IsValid() {
		return validationError(form.Validator)
	}

	// Verify token authentication
	token := app.sessionManager.GetString(r.Context(), verificationTokenSessionKey)
	if token == "" {
		return respErr{statusCode: http.StatusUnauthorized}
	}

	err = app.models.Verification.Verify(token, email)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			return respErr{statusCode: http.StatusUnauthorized}
		}
		if errors.Is(err, models.ErrExpiredVerification) {
			app.flash(r, ExpiredTokenFlash)
			http.Redirect(w, r, "/", http.StatusSeeOther)

			return nil
		}

		return err
	}

	// Upon registration, purge db of all verifications with email.
	err = app.models.Verification.Purge(email)
	if err != nil {
		return err
	}

	userID, err := app.models.User.Insert(email, form.password)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateEmail) {
			return respErr{statusCode: http.StatusUnauthorized}
		}

		return err
	}

	// Login user
	app.sessionManager.Clear(r.Context())
	err = app.login(r, userID)
	if err != nil {
		return err
	}

	f := FlashMessage{
		Type:    FlashSuccess,
		Message: "Successfully created account. Welcome!",
	}
	app.flash(r, f)
	http.Redirect(w, r, "/", http.StatusSeeOther)

	return nil
}

func (app *application) handleAuthResetGet(w http.ResponseWriter, r *http.Request) error {
	return app.render(w, r, http.StatusOK, "auth-reset.tmpl", nil)
}

type authResetForm struct {
	email string
	validator.Validator
}

func (app *application) handleAuthResetPost(w http.ResponseWriter, r *http.Request) error {
	var email string

	// Get users email if already authenticated.
	if app.isAuthenticated(r) {
		suid, err := app.getSessionUserID(r)
		if err != nil {
			return err
		}

		u, err := app.models.User.GetProfile(suid)
		if err != nil {
			return err
		}

		email = u.Email
	} else {
		// If not authenticated, parse form and validate email address
		err := r.ParseForm()
		if err != nil {
			return respErr{
				statusCode: http.StatusBadRequest,
				message:    "unable to parse form",
			}
		}

		form := authResetForm{email: r.Form.Get("email")}

		form.Validate(validator.NotBlank(form.email), "invalid email: cannot be blank")
		form.Validate(validator.MaxChars(form.email, 254), "invalid email: must be no more than 254 characters long")

		if !form.IsValid() {
			return validationError(form.Validator)
		}

		email = form.email
	}

	exists, err := app.models.User.ExistsEmail(email)
	if err != nil {
		return err
	}

	f := FlashMessage{
		Type:    FlashInfo,
		Message: "A link to reset your password has been sent to the email address provided. Please check your junk folder.",
	}

	// If user does not exist, respond with consistent flash message
	if !exists {
		app.flash(r, f)
		app.refresh(w, r)

		return nil
	}

	// Check if link verification has already been created
	v, err := app.models.Verification.Get(email)
	if err != nil && err != models.ErrNoRecord {
		return err
	}

	// Don't send a new link if less than 5 minutes since last
	if v != nil {
		if time.Since(v.CreatedAt) < 5*time.Minute {
			app.flash(r, f)
			app.refresh(w, r)

			return nil
		}
	}

	token, err := app.models.Verification.New(email)
	if err != nil {
		return err
	}

	// Create link with token
	ref, err := url.Parse("/auth/reset/update")
	if err != nil {
		return err
	}
	q := ref.Query()
	q.Set("token", token)
	ref.RawQuery = q.Encode()
	link := app.baseURL.ResolveReference(ref)

	// Send mail in background routine
	if !app.config.dev {
		app.background(func() {
			err = app.mailer.Send(email, "email-verification.tmpl", link)
			if err != nil {
				app.logger.Error("mailer", slog.Any("err", err))
			}
		})
	}
	app.logger.Debug("mailed", slog.String("link", link.String()))

	app.sessionManager.RenewToken(r.Context())
	app.sessionManager.Put(r.Context(), resetEmailSessionKey, email)

	app.flash(r, f)
	app.refresh(w, r)

	return nil
}

type resetUpdateData struct {
	HasSessionEmail bool
}

func (app *application) handleAuthResetUpdateGet(w http.ResponseWriter, r *http.Request) error {
	queryToken := r.URL.Query().Get("token")
	if queryToken == "" {
		return respErr{statusCode: http.StatusUnauthorized}
	}

	app.sessionManager.Put(r.Context(), resetTokenSessionKey, queryToken)

	hasSessionEmail := app.sessionManager.Exists(r.Context(), resetEmailSessionKey)
	data := resetUpdateData{
		HasSessionEmail: hasSessionEmail,
	}

	return app.render(w, r, http.StatusOK, "auth-reset-update.tmpl", data)
}

type authResetUpdateForm struct {
	email    string
	password string
	validator.Validator
}

func (app *application) handleAuthResetUpdatePost(w http.ResponseWriter, r *http.Request) error {
	err := r.ParseForm()
	if err != nil {
		return respErr{
			statusCode: http.StatusBadRequest,
			message:    "unable to parse form",
		}
	}

	form := authResetUpdateForm{
		email:    r.Form.Get("email"),
		password: r.Form.Get("password"),
	}

	email := app.sessionManager.GetString(r.Context(), resetEmailSessionKey)
	if form.email != "" {
		email = form.email
	}

	form.Validate(validator.NotBlank(email), "invalid email: cannot be blank")
	form.Validate(validator.Matches(email, validator.EmailRX), "invalid email: must be a valid email address")
	form.Validate(validator.MaxChars(email, 254), "invalid email: must be no more than 254 characters long")
	form.Validate(validator.NotBlank(form.password), "invalid password: cannot be blank")
	form.Validate(validator.MinChars(form.password, 8), "invalid password: must be at least 8 characters long")
	form.Validate(validator.MaxChars(form.password, 72), "invalid password: must be no more than 72 characters long")

	if !form.IsValid() {
		return validationError(form.Validator)
	}

	token := app.sessionManager.GetString(r.Context(), resetTokenSessionKey)
	if token == "" {
		return respErr{statusCode: http.StatusUnauthorized}
	}

	err = app.models.Verification.Verify(token, email)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			return respErr{statusCode: http.StatusUnauthorized}
		}
		if errors.Is(err, models.ErrExpiredVerification) {
			app.flash(r, ExpiredTokenFlash)
			http.Redirect(w, r, "/", http.StatusSeeOther)

			return nil
		}

		return err
	}

	err = app.models.Verification.Purge(email)
	if err != nil {
		return err
	}

	err = app.models.User.UpdatePassword(email, form.password)
	if err != nil {
		return err
	}

	app.sessionManager.Clear(r.Context())

	f := FlashMessage{
		Type:    FlashSuccess,
		Message: "Successfully updated password. Please login.",
	}
	app.flash(r, f)

	http.Redirect(w, r, "/", http.StatusSeeOther)

	return nil
}

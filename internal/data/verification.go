package data

import (
	"context"
	"errors"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	VerificationTokenTTL = time.Hour * 36
	ScopeRegistration    = "registration"
	ScopeChangeEmail     = "change_email"
	ScopeResetPassword   = "reset_password"
	ScopeAccountDeletion = "account_deletion"
)

type VerificationTokenModel struct {
	pool *pgxpool.Pool
}

type VerificationToken struct {
	Scope  string
	Email  string
	UserID *uuid.UUID
	*Token
}

func (vt VerificationToken) Validate() error {
	return validation.ValidateStruct(&vt,
		validation.Field(&vt.Hash, validation.Required),
		validation.Field(&vt.Expiry, validation.Required),
		validation.Field(&vt.Email, validation.Required, is.Email))
}

// Create and insert new verification for email. Generates a randomly
// generated token and stores a hash of it in the database. Returns
// the plaintext token.
func (m VerificationTokenModel) New(scope, email string, userID *uuid.UUID) (*Token, error) {
	t, err := generateToken(VerificationTokenTTL)
	if err != nil {
		return nil, err
	}

	vt := &VerificationToken{
		Scope:  scope,
		Email:  email,
		UserID: userID,
		Token:  t,
	}

	err = m.Insert(vt)
	if err != nil {
		return nil, err
	}

	return t, err
}

func (m VerificationTokenModel) Insert(vt *VerificationToken) error {
	err := vt.Validate()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		INSERT INTO verification_token_ (hash_, expiry_, scope_, email_, user_id_)
		VALUES($1, $2, $3, $4, $5);`

	args := []any{vt.Hash, vt.Expiry, vt.Scope, vt.Email, vt.UserID}

	_, err = m.pool.Exec(ctx, sql, args...)
	return err
}

func (m VerificationTokenModel) Exists(scope, email string, userID *uuid.UUID) (bool, error) {
	var exists bool

	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM verification_token_
			WHERE scope_ = $1
			AND email_ = $2`

	args := []any{scope, email}

	if userID != nil {
		sql += `
			AND user_id_ = $3`
		args = append(args, *userID)
	}
	sql += `
		);`

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	err := m.pool.QueryRow(ctx, sql, args...).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (m VerificationTokenModel) PurgeWithEmail(email string) error {
	sql := `
		DELETE FROM verification_token_
		WHERE email_ = $1;`

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	_, err := m.pool.Exec(ctx, sql, email)
	return err
}

func (m VerificationTokenModel) PurgeWithUserID(userID uuid.UUID) error {
	sql := `
		DELETE FROM verification_token_
		WHERE user_id_ = $1;`

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	_, err := m.pool.Exec(ctx, sql, userID)
	return err
}

func (m VerificationTokenModel) Verify(token, scope, email string, userID *uuid.UUID) error {
	var expiry time.Time

	sql := `
		SELECT expiry_
		FROM verification_token_
		WHERE hash_ = $1
		AND scope_ = $2
		AND email_ = $3`

	hash := generateHash(token)
	args := []any{hash, scope, email}

	if userID != nil {
		sql += `
		AND user_id_ = $4`
		args = append(args, *userID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	err := m.pool.QueryRow(ctx, sql, args...).Scan(&expiry)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrRecordNotFound
		default:
			return err
		}
	}

	if time.Now().After(expiry) {
		return ErrExpiredToken
	}

	return nil
}

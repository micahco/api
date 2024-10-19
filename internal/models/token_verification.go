package models

import (
	"context"
	"errors"
	"time"

	val "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Default expiry duration
const VerificationTokenTTL = time.Hour * 36

type VerificationTokenModel struct {
	pool *pgxpool.Pool
}

type VerificationToken struct {
	Email string
	*Token
}

func (vt VerificationToken) Validate() error {
	return val.ValidateStruct(&vt,
		val.Field(&vt.Hash, val.Required),
		val.Field(&vt.Email, val.Required, is.Email),
		val.Field(&vt.Expiry, val.Required))
}

// Create and insert new verification for email. Generates a randomly
// generated token and stores a hash of it in the database. Returns
// the plaintext token.
func (m VerificationTokenModel) New(email string) (*Token, error) {
	t, err := generateToken(VerificationTokenTTL)
	if err != nil {
		return nil, err
	}

	vt := &VerificationToken{email, t}

	err = m.Insert(vt)
	if err != nil {
		return nil, err
	}

	return t, err
}

func (m VerificationTokenModel) Insert(t *VerificationToken) error {
	err := t.Validate()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		INSERT INTO verification_token_ (hash_, email_, expiry_)
		VALUES($1, $2, $3);`

	args := []any{t.Hash, t.Email, t.Expiry}

	_, err = m.pool.Exec(ctx, sql, args...)
	return err
}

func (m VerificationTokenModel) Exists(email string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM verification_token_
			WHERE email_ = $1
		);`

	var exists bool
	err := m.pool.QueryRow(ctx, sql, email).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (m VerificationTokenModel) Purge(email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		DELETE FROM verification_token_
		WHERE email_ = $1;`

	_, err := m.pool.Exec(ctx, sql, email)
	return err
}

func (m VerificationTokenModel) Verify(email, token string) error {
	hash := generateHash(token)

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		SELECT expiry_
		FROM verification_token_
		WHERE hash_ = $1
		AND email_ = $2;`

	args := []any{hash, email}

	var expiry time.Time
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

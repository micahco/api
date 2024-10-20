package data

import (
	"context"
	"errors"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Default expiry duration
const AuthenticationTokenTTL = time.Hour * 36

type AuthenticationTokenModel struct {
	pool *pgxpool.Pool
}

type AuthenticationToken struct {
	UserID uuid.UUID
	*Token
}

func (at AuthenticationToken) Validate() error {
	return validation.ValidateStruct(&at,
		validation.Field(&at.Hash, validation.Required),
		validation.Field(&at.Expiry, validation.Required),
		validation.Field(&at.UserID, validation.Required))
}

func (m AuthenticationTokenModel) New(userID uuid.UUID) (*Token, error) {
	t, err := generateToken(AuthenticationTokenTTL)
	if err != nil {
		return nil, err
	}

	at := &AuthenticationToken{userID, t}

	err = m.Insert(at)
	if err != nil {
		return nil, err
	}

	return t, err
}

func (m AuthenticationTokenModel) Insert(t *AuthenticationToken) error {
	err := t.Validate()
	if err != nil {
		return err
	}

	sql := `
		INSERT INTO authentication_token_ (hash_, expiry_, user_id_)
		VALUES($1, $2, $3);`

	args := []any{t.Hash, t.Expiry, t.UserID}

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	_, err = m.pool.Exec(ctx, sql, args...)
	return err
}

func (m AuthenticationTokenModel) Exists(email string) (bool, error) {
	var exists bool

	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM authentication_token_
			WHERE email_ = $1
		);`

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	err := m.pool.QueryRow(ctx, sql, email).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (m AuthenticationTokenModel) Purge(email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		DELETE FROM authentication_token_
		WHERE email_ = $1;`

	_, err := m.pool.Exec(ctx, sql, email)
	return err
}

func (m AuthenticationTokenModel) Verify(email, token string) error {
	var expiry time.Time

	sql := `
		SELECT expiry_
		FROM authentication_token_
		WHERE hash_ = $1
		AND email_ = $2;`

	hash := generateHash(token)
	args := []any{hash, email}

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

package data

import (
	"context"
	"errors"
	"time"

	val "github.com/go-ozzo/ozzo-validation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Default expiry duration
const AuthenticationTokenTTL = time.Hour * 36

type AuthenticationTokenModel struct {
	pool *pgxpool.Pool
}

type AuthenticationToken struct {
	UserID int64
	*Token
}

func (at AuthenticationToken) Validate() error {
	return val.ValidateStruct(&at,
		val.Field(&at.Hash, val.Required),
		val.Field(&at.UserID, val.Required),
		val.Field(&at.Expiry, val.Required))
}

func (m AuthenticationTokenModel) New(userID int64) (*Token, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		INSERT INTO authentication_token_ (hash_, user_id_, expiry_)
		VALUES($1, $2, $3);`

	args := []any{t.Hash, t.UserID, t.Expiry}

	_, err = m.pool.Exec(ctx, sql, args...)
	return err
}

func (m AuthenticationTokenModel) Exists(email string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM authentication_token_
			WHERE email_ = $1
		);`

	var exists bool
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
	hash := generateHash(token)

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		SELECT expiry_
		FROM authentication_token_
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

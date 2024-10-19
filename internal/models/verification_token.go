package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"time"

	val "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Default expiry duration
const ttl = time.Hour * 36

type VerificationTokenModel struct {
	pool *pgxpool.Pool
}

type Verification struct {
	Hash   []byte
	Email  string
	Expiry time.Time
}

func (v Verification) Validate() error {
	return val.ValidateStruct(&v,
		val.Field(&v.Hash, val.Required),
		val.Field(&v.Email, val.Required, is.Email),
		val.Field(&v.Expiry, val.Required))
}

// Create and insert new verification for email. Returns the plaintext token
func (m VerificationTokenModel) New(email string) (string, error) {
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	token := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)
	// Remember: this is a byte array and must be converted into a slice [:]
	hash := sha256.Sum256([]byte(token))

	v := &Verification{
		Hash:   hash[:],
		Email:  email,
		Expiry: time.Now().Add(ttl),
	}
	err = m.Insert(v)
	if err != nil {
		return "", err
	}

	return token, err
}

func (m VerificationTokenModel) Insert(v *Verification) error {
	err := v.Validate()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		INSERT INTO verification_token_ (hash_, email_, expiry_)
		VALUES($1, $2, $3);`

	args := []any{v.Hash, v.Email, v.Expiry}

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
	hash := sha256.Sum256([]byte(token))

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		SELECT expiry_
		FROM verification_token_
		WHERE hash_ = $1
		AND email_ = $2;`

	args := []any{hash[:], email}

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
		return ErrExpiredVerification
	}

	return nil
}

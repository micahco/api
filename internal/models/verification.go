package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VerificationModel struct {
	pool *pgxpool.Pool
}

type Verification struct {
	Plaintext string
	Hash      []byte
	Email     string
	Expiry    time.Time
}

func (m VerificationModel) New(email string) (*Verification, error) {
	ttl := time.Hour * 36
	v, err := generateVerification(email, ttl)
	if err != nil {
		return nil, err
	}

	err = m.Insert(v)
	return v, err
}

func (m VerificationModel) Insert(v *Verification) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sql := `
		INSERT INTO verification_ (hash_, email_, expiry_)
		VALUES($1, $2, $3);`

	args := []interface{}{v.Hash, v.Email, v.Expiry}

	_, err := m.pool.Exec(ctx, sql, args...)
	return err
}

func (m VerificationModel) Purge(email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sql := `
		DELETE FROM verification_
		WHERE email_ = $1;`

	_, err := m.pool.Exec(ctx, sql, email)
	return err
}

func (m VerificationModel) Verify(email, token string) error {
	hash := sha256.Sum256([]byte(token))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sql := `
		SELECT expiry_
		FROM verification_
		WHERE hash_ = $1
		AND email_ = $2;`

	args := []interface{}{hash[:], email}

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

func generateVerification(email string, ttl time.Duration) (*Verification, error) {
	v := &Verification{
		Email:  email,
		Expiry: time.Now().Add(ttl),
	}

	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	v.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	hash := sha256.Sum256([]byte(v.Plaintext))
	v.Hash = hash[:]

	return v, nil
}

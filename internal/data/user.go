package data

import (
	"context"
	"errors"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserModel struct {
	pool *pgxpool.Pool
}

type User struct {
	ID           int       `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	Email        string    `json:"email"`
	PasswordHash []byte    `json:"-"`
	Version      int       `json:"-"`
}

var AnonymousUser = &User{}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

func (m UserModel) New(email, password string) (*User, error) {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return nil, err
	}

	user := &User{
		Email:        email,
		PasswordHash: []byte(hash),
	}

	err = m.Insert(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (m UserModel) Insert(user *User) error {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		INSERT INTO user_ (email_, password_hash_)
		VALUES($1, $2)
		RETURNING id_, created_at_, version_;`

	args := []any{user.Email, user.PasswordHash}

	err := m.pool.QueryRow(ctx, sql, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		switch {
		case pgErrCode(err) == pgerrcode.UniqueViolation:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

func (m UserModel) GetForCredentials(email, password string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		SELECT id_, created_at_, email_, password_hash_, version_
		FROM user_ WHERE email_ = $1;`

	var u User
	err := m.pool.QueryRow(ctx, sql, email).Scan(
		&u.ID,
		&u.CreatedAt,
		&u.Email,
		&u.PasswordHash,
		&u.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrInvalidCredentials
		default:
			return nil, err
		}
	}

	match, err := argon2id.ComparePasswordAndHash(password, string(u.PasswordHash))
	if err != nil {
		return nil, err
	}
	if !match {
		return nil, ErrInvalidCredentials
	}

	return &u, nil
}

func (m UserModel) GetForAuthToken(token string) (*User, error) {
	hash := generateHash(token)

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		SELECT user_.id_, user_.created_at_, user_.email_, user_.password_hash_, 
		user_.version_, authentication_token_.expiry_
		FROM user_
		INNER JOIN authentication_token_
		ON user_.id_ = authentication_token_.user_id_
		WHERE authentication_token_.hash_ = $1;`

	var u User
	var expiry time.Time
	err := m.pool.QueryRow(ctx, sql, hash).Scan(
		&u.ID,
		&u.CreatedAt,
		&u.Email,
		&u.PasswordHash,
		&u.Version,
		&expiry,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	if time.Now().After(expiry) {
		return nil, ErrExpiredToken
	}

	return &u, nil
}

func (m UserModel) Exists(email string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM user_
			WHERE email_ = $1
		);`

	var exists bool
	err := m.pool.QueryRow(ctx, sql, email).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (m UserModel) Update(user *User) error {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	sql := `
		UPDATE users 
        SET email_ = $1, password_hash_ = $2, version_ = version_ + 1
        WHERE id_ = $3 AND version_ = $4
        RETURNING version`

	args := []any{
		user.Email,
		user.PasswordHash,
		user.ID,
		user.Version,
	}

	err := m.pool.QueryRow(ctx, sql, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case pgErrCode(err) == pgerrcode.UniqueViolation:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

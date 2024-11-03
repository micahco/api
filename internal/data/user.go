package data

import (
	"context"
	"errors"
	"time"

	"github.com/alexedwards/argon2id"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserModel struct {
	pool *pgxpool.Pool
}

type User struct {
	ID           uuid.UUID `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	Email        string    `json:"email"`
	PasswordHash []byte    `json:"-"`
	Version      int       `json:"-"`
}

var AnonymousUser = &User{}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

func (u User) Validate() error {
	return validation.ValidateStruct(&u,
		validation.Field(&u.Email, validation.Required, is.Email),
		validation.Field(&u.PasswordHash, validation.Required))
}

func (u *User) SetPasswordHash(password string) error {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return err
	}

	u.PasswordHash = []byte(hash)

	return nil
}

func (m UserModel) New(email, password string) (*User, error) {
	user := &User{Email: email}

	err := user.SetPasswordHash(password)
	if err != nil {
		return nil, err
	}

	err = m.Insert(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (m UserModel) Insert(user *User) error {
	err := user.Validate()
	if err != nil {
		return err
	}

	sql := `
		INSERT INTO user_ (email_, password_hash_)
		VALUES($1, $2)
		RETURNING id_, created_at_, version_;`

	args := []any{user.Email, user.PasswordHash}

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	err = m.pool.QueryRow(ctx, sql, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
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
	var u User

	sql := `
		SELECT id_, created_at_, email_, password_hash_, version_
		FROM user_ WHERE email_ = $1;`

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

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

func (m UserModel) GetForAuthenticationToken(token string) (*User, error) {
	var u User
	var expiry time.Time

	sql := `
		SELECT user_.id_, user_.created_at_, user_.email_, user_.password_hash_, 
		user_.version_, authentication_token_.expiry_
		FROM user_
		INNER JOIN authentication_token_
		ON user_.id_ = authentication_token_.user_id_
		WHERE authentication_token_.hash_ = $1;`

	hash := generateHash(token)

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

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

func (m UserModel) GetForVerificationToken(scope, token string) (*User, error) {
	var u User
	var expiry time.Time

	sql := `
		SELECT user_.id_, user_.created_at_, user_.email_, user_.password_hash_, 
		user_.version_, verification_token_.expiry_
		FROM user_
		INNER JOIN verification_token_
		ON user_.id_ = verification_token_.user_id_
		WHERE verification_token_.scope_ = $1
		AND verification_token_.hash_ = $2;`

	hash := generateHash(token)
	args := []any{scope, hash}

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	err := m.pool.QueryRow(ctx, sql, args...).Scan(
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

func (m UserModel) ExistsWithEmail(email string) (bool, error) {
	var exists bool

	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM user_
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

func (m UserModel) Update(user *User) error {
	err := user.Validate()
	if err != nil {
		return err
	}

	sql := `
		UPDATE user_ 
        SET email_ = $1, password_hash_ = $2, version_ = version_ + 1
        WHERE id_ = $3 AND version_ = $4
        RETURNING version_`

	args := []any{
		user.Email,
		user.PasswordHash,
		user.ID,
		user.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	err = m.pool.QueryRow(ctx, sql, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrEditConflict
		case pgErrCode(err) == pgerrcode.UniqueViolation:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

package data

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ctxTimeout = 3 * time.Second

type Models struct {
	User                UserModel
	VerificationToken   VerificationTokenModel
	AuthenticationToken AuthenticationTokenModel
}

func New(pool *pgxpool.Pool) Models {
	return Models{
		User:                UserModel{pool},
		VerificationToken:   VerificationTokenModel{pool},
		AuthenticationToken: AuthenticationTokenModel{pool},
	}
}

var (
	ErrRecordNotFound     = errors.New("models: no matching record found")
	ErrInvalidCredentials = errors.New("models: invalid credentials")
	ErrDuplicateEmail     = errors.New("models: duplicate email")
	ErrExpiredToken       = errors.New("models: expired token")
	ErrEditConflict       = errors.New("models: edit conflict")
)

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}

	return ""
}

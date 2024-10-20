package data

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrRecordNotFound     = errors.New("models: no matching record found")
	ErrInvalidCredentials = errors.New("models: invalid credentials")
	ErrDuplicateEmail     = errors.New("models: duplicate email")
	ErrExpiredToken       = errors.New("models: expired token")
)

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}

	return ""
}

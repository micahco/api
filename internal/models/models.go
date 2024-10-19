package models

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const ctxTimeout = 3 * time.Second

type Models struct {
	User              UserModel
	VerificationToken VerificationTokenModel
}

func New(pool *pgxpool.Pool) Models {
	return Models{
		User:              UserModel{pool},
		VerificationToken: VerificationTokenModel{pool},
	}
}

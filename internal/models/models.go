package models

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const ctxTimeout = 3 * time.Second

type Models struct {
	User         UserModel
	Verification VerificationModel
}

func New(pool *pgxpool.Pool) Models {
	return Models{
		User:         UserModel{pool},
		Verification: VerificationModel{pool},
	}
}

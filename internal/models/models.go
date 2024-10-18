package models

import "github.com/jackc/pgx/v5/pgxpool"

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

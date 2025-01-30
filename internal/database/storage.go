package database

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Database interface {
	GetDB() *sqlx.DB
	GetReadDB() *sqlx.DB
	BeginTx(context.Context) (*sqlx.Tx, error)
	Rollback(tx *sqlx.Tx, err error)
	Close() error
}

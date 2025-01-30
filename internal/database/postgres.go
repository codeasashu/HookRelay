package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type Postgres struct {
	id   int
	db   *sqlx.DB
	pool *pgxpool.Pool

	replicas []*Postgres
	randGen  *rand.Rand
}

func NewPostgreSQLStorage() (*Postgres, error) {
	dbConfig := config.HRConfig.Database
	primary, err := parseDBConfig(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %v", err)
	}

	primary.id = 0
	replicas := make([]*Postgres, 0)
	for i, replica := range dbConfig.ReadReplicas {
		if replica.Scheme == "" {
			replica.Scheme = dbConfig.Scheme
		}
		r, e := parseDBConfig(replica)
		if e != nil {
			return nil, e
		}
		r.id = i + 1
		replicas = append(replicas, r)
	}
	primary.replicas = replicas
	primary.randGen = rand.New(rand.NewSource(time.Now().UnixNano()))

	if err_ := ping(primary); err_ != nil {
		return nil, err_
	}

	return primary, err
}

func parseDBConfig(dbConfig config.DatabaseConfiguration) (*Postgres, error) {
	pgxCfg, err := pgxpool.ParseConfig(dbConfig.BuildDsn())
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if dbConfig.SetMaxOpenConnections > 0 {
		pgxCfg.MaxConns = int32(dbConfig.SetMaxOpenConnections)
	}
	pgxCfg.MaxConnLifetime = time.Second * time.Duration(dbConfig.SetConnMaxLifetime)

	pool, err := pgxpool.NewWithConfig(context.Background(), pgxCfg)
	if err != nil {
		defer pool.Close()
		return nil, fmt.Errorf("[postgres]: failed to open database - %v", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	db := sqlx.NewDb(sqlDB, "pgx")

	return &Postgres{db: db, pool: pool}, nil
}

func (p *Postgres) GetDB() *sqlx.DB {
	return p.db
}

func (p *Postgres) GetReadDB() *sqlx.DB {
	if len(p.replicas) > 0 {
		r, err := p.getRandomReplica()
		if err != nil || r == nil {
			id := ""
			if r != nil {
				id = fmt.Sprintf(" %d", r.id)
			}
			slog.Error("failed to get random replica", "id", id, "err", err)
			return p.db
		}
		slog.Debug("fetched replica", "id", r.id)
		return r.db
	}
	return p.db
}

func (p *Postgres) getRandomReplica() (*Postgres, error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occurred: %v", r)
		}
	}()
	return p.replicas[p.randGen.Intn(len(p.replicas))], err
}

func (p *Postgres) Close() error {
	p.pool.Close()
	return p.db.Close()
}

func (p *Postgres) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return p.db.BeginTxx(ctx, nil)
}

func (p *Postgres) Rollback(tx *sqlx.Tx, err error) {
	if err != nil {
		rbErr := tx.Rollback()
		slog.Error("failed to roll back transaction in ProcessBroadcastEventCreation", "err", rbErr)
	}

	cmErr := tx.Commit()
	if cmErr != nil && !errors.Is(cmErr, sql.ErrTxDone) {
		slog.Error("failed to commit tx in ProcessBroadcastEventCreation, rolling back transaction", "err", cmErr)
		rbErr := tx.Rollback()
		slog.Error("failed to roll back transaction in ProcessBroadcastEventCreation", "err", rbErr)
	}
}

func ping(p *Postgres) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err := p.Ping(ctx)
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(len(p.replicas)+1)*5*time.Second)
	defer cancel()
	err = p.PingReplicas(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (p *Postgres) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

func (p *Postgres) PingReplicas(ctx context.Context) error {
	for _, replica := range p.replicas {
		if err := replica.db.PingContext(ctx); err != nil {
			slog.Error("replica ping failed", "id", replica.id, "err", err)
			return err
		}
	}
	return nil
}

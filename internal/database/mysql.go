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
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type MySQL struct {
	id int
	db *sqlx.DB

	replicas []*MySQL
	randGen  *rand.Rand
}

func NewMySQLStorage() (*MySQL, error) {
	dbConfig := config.HRConfig.Database
	primary, err := parseMysqlDBConfig(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %v", err)
	}

	primary.id = 0
	replicas := make([]*MySQL, 0)
	for i, replica := range dbConfig.ReadReplicas {
		if replica.Scheme == "" {
			replica.Scheme = dbConfig.Scheme
		}
		r, e := parseMysqlDBConfig(replica)
		if e != nil {
			return nil, e
		}
		r.id = i + 1
		replicas = append(replicas, r)
	}
	primary.replicas = replicas
	primary.randGen = rand.New(rand.NewSource(time.Now().UnixNano()))

	if err_ := pinger(primary); err_ != nil {
		return nil, err_
	}

	return primary, err
}

func parseMysqlDBConfig(dbConfig config.DatabaseConfiguration) (*MySQL, error) {
	cfg := mysql.Config{
		User:                 dbConfig.Username,
		Passwd:               dbConfig.Password,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", dbConfig.Host, dbConfig.Port),
		DBName:               dbConfig.Database,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	slog.Info("Connecting to mysql...")
	db, err := sqlx.Connect("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	if dbConfig.SetMaxOpenConnections > 0 {
		db.SetMaxOpenConns(dbConfig.SetMaxOpenConnections)
	}
	db.SetConnMaxLifetime(time.Second * time.Duration(dbConfig.SetConnMaxLifetime))

	return &MySQL{db: db}, nil
}

func (m *MySQL) GetDB() *sqlx.DB {
	return m.db
}

func (m *MySQL) GetReadDB() *sqlx.DB {
	if len(m.replicas) > 0 {
		r, err := m.getRandomReplica()
		if err != nil || r == nil {
			id := ""
			if r != nil {
				id = fmt.Sprintf(" %d", r.id)
			}
			slog.Error("failed to get random replica", "id", id, "err", err)
			return m.db
		}
		slog.Debug("fetched replica", "id", r.id)
		return r.db
	}
	return m.db
}

func (m *MySQL) getRandomReplica() (*MySQL, error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occurred: %v", r)
		}
	}()
	return m.replicas[m.randGen.Intn(len(m.replicas))], err
}

func (m *MySQL) Close() error {
	return m.db.Close()
}

func (m *MySQL) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return m.db.BeginTxx(ctx, nil)
}

func (m *MySQL) Rollback(tx *sqlx.Tx, err error) {
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

func pinger(m *MySQL) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err := m.Ping(ctx)
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(len(m.replicas)+1)*5*time.Second)
	defer cancel()
	err = m.PingReplicas(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (m *MySQL) Ping(ctx context.Context) error {
	return m.db.PingContext(ctx)
}

func (m *MySQL) PingReplicas(ctx context.Context) error {
	for _, replica := range m.replicas {
		if err := replica.db.PingContext(ctx); err != nil {
			slog.Error("replica ping failed", "id", replica.id, "err", err)
			return err
		}
	}
	return nil
}

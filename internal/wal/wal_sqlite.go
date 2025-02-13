package wal

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	sync "sync"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/event"
	_ "github.com/mattn/go-sqlite3"
)

type WALSQLite struct {
	logDir     string
	curDB      *sql.DB
	mu         sync.Mutex
	accounting *Accounting
}

func NewSQLiteWAL(a *Accounting) *WALSQLite {
	return &WALSQLite{
		logDir:     config.HRConfig.WalConfig.Path,
		accounting: a,
	}
}

func (w *WALSQLite) Init(t time.Time) error {
	slog.Debug("initializing WAL at", slog.Any("log-dir", w.logDir))
	if err := os.MkdirAll(w.logDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	timestamp := t.Format(config.HRConfig.WalConfig.Format)
	path := filepath.Join(w.logDir, fmt.Sprintf("wal_%s.sqlite3", timestamp))

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return err
	}

	// @TODO: use binary/JSONB for payload (see: https://sqlite.org/draft/jsonb.html)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS wal_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
        owner_id TEXT NOT NULL,
        event_type TEXT NOT NULL,
        payload TEXT, -- SQLite does not have a native JSON type, store as TEXT
        idempotency_key TEXT,
        accepted_at TIMESTAMP NULL DEFAULT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS wal_event_deliveries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
        event_type TEXT NOT NULL,
        payload TEXT NOT NULL,
        subscription_id TEXT NOT NULL,
        status_code INTEGER,
        error TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)
	if err != nil {
		return err
	}
	w.curDB = db
	return nil
}

func (w *WALSQLite) LogEvent(e *event.Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	payloadBytes, err := json.Marshal(e.Payload)
	if err != nil {
		payloadBytes = []byte("{}")
	}

	if res, err := w.curDB.Exec("INSERT INTO wal_events (owner_id, event_type, payload) VALUES (?,?,?)", e.OwnerId, e.EventType, payloadBytes); err != nil {
		slog.Error("failed to log event in WAL", slog.Any("error", err))
		return err
	} else {
		id, err := res.LastInsertId()
		if err == nil {
			e.UID = id
		} else {
			e.UID = -1
		}
		slog.Debug("logged event in WAL", slog.Any("id", e.UID))
	}
	return nil
}

func (w *WALSQLite) LogEventDelivery(e *event.EventDelivery) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	payloadBytes, err := json.Marshal(e.Payload)
	if err != nil {
		payloadBytes = []byte("{}")
	}

	if res, err := w.curDB.Exec("INSERT INTO wal_event_deliveries (event_type, payload, subscription_id, status_code, error, created_at) VALUES (?,?,?,?,?,?)", e.EventType, payloadBytes, e.SubscriptionId, e.StatusCode, e.Error, e.CompletedAt); err != nil {
		slog.Error("failed to log event in WAL", slog.Any("error", err))
		return err
	} else {
		id, err := res.LastInsertId()
		if err == nil {
			e.ID = id
		} else {
			e.ID = -1
		}
		slog.Debug("logged event in WAL", slog.Any("id", e.ID))
	}
	return nil
}

func (w *WALSQLite) LogBatchEventDelivery(events []*event.EventDelivery) error {
	if len(events) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	tx, err := w.curDB.Begin()
	if err != nil {
		slog.Error("failed to start transaction for batch event logging", slog.Any("error", err))
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO wal_event_deliveries (event_type, payload, subscription_id, status_code, error, created_at) VALUES (?,?,?,?,?,?)")
	if err != nil {
		slog.Error("failed to prepare statement for batch event logging", slog.Any("error", err))
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, e := range events {
		_, err := stmt.Exec(e.EventType, e.Payload, e.SubscriptionId, e.StatusCode, e.Error, e.CompletedAt)
		if err != nil {
			slog.Error("failed to log event in batch WAL insert", slog.Any("error", err))
			_ = tx.Rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit batch WAL insert", slog.Any("error", err))
		return err
	}

	slog.Debug("batch logged events in WAL", "count", len(events))
	return nil
}

func (w *WALSQLite) Close() error {
	// if w.accounting != nil {
	// 	w.accounting.db.Close()
	// }
	return w.curDB.Close()
}

func (w *WALSQLite) Shutdown() error {
	slog.Info("Shutting down delivery db")
	if w.accounting != nil {
		w.accounting.db.Close()
	}
	return w.curDB.Close()
}

func (w *WALSQLite) ForEachEvent(f func(c event.Event) error) error {
	files, err := os.ReadDir(w.logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %v", err)
	}

	var walFiles []os.DirEntry

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".sqlite3" {
			walFiles = append(walFiles, file)
		}
	}

	if len(walFiles) == 0 {
		return fmt.Errorf("no valid WAL files found in log directory")
	}

	// Sort files by timestamp in ascending order
	sort.Slice(walFiles, func(i, j int) bool {
		timestampStrI := walFiles[i].Name()[4:17]
		timestampStrJ := walFiles[j].Name()[4:17]
		timestampI, errI := time.Parse(config.HRConfig.WalConfig.Format, timestampStrI)
		timestampJ, errJ := time.Parse(config.HRConfig.WalConfig.Format, timestampStrJ)
		if errI != nil || errJ != nil {
			return false
		}
		return timestampI.Before(timestampJ)
	})

	for _, file := range walFiles {
		filePath := filepath.Join(w.logDir, file.Name())

		slog.Debug("loading WAL", slog.Any("file", filePath))

		db, err := sql.Open("sqlite3", filePath)
		if err != nil {
			return fmt.Errorf("failed to open WAL file %s: %v", file.Name(), err)
		}

		rows, err := db.Query("SELECT command FROM wal")
		if err != nil {
			return fmt.Errorf("failed to query WAL file %s: %v", file.Name(), err)
		}

		for rows.Next() {
			var e event.Event
			var payload string
			// if err := rows.Scan(&command); err != nil {
			if err := rows.Scan(&e.UID, &e.OwnerId, &e.EventType, &payload, &e.CreatedAt, &e.AcknowledgedAt, &e.CompletedAt); err != nil {
				return fmt.Errorf("failed to scan WAL file %s: %v", file.Name(), err)
			}

			if err := json.Unmarshal([]byte(payload), &e.Payload); err != nil {
				slog.Error("failed to unmarshal payload", slog.Any("error", err))
				continue
			}
			if err := f(e); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed to iterate WAL file %s: %v", file.Name(), err)
		}

		if err := db.Close(); err != nil {
			return fmt.Errorf("failed to close WAL file %s: %v", file.Name(), err)
		}
	}

	return nil
}

func (w *WALSQLite) ForEachEventDeliveriesBatch(batchSize int, f func([]*event.EventDelivery) error) error {
	files, err := os.ReadDir(w.logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %v", err)
	}

	var walFiles []os.DirEntry
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".sqlite3" {
			walFiles = append(walFiles, file)
		}
	}

	if len(walFiles) == 0 {
		return fmt.Errorf("no valid WAL files found in log directory")
	}

	walFileLen := len(config.HRConfig.WalConfig.Format) + 4 // include prefix in overall len

	currTime := time.Now().Format(config.HRConfig.WalConfig.Format)
	// Sort files by timestamp in ascending order
	sort.Slice(walFiles, func(i, j int) bool {
		timestampStrI := walFiles[i].Name()[4:walFileLen]
		timestampStrJ := walFiles[j].Name()[4:walFileLen]
		// Skip the currently processing files
		if timestampStrI == currTime || timestampStrJ == currTime {
			return false
		}
		timestampI, errI := time.Parse(config.HRConfig.WalConfig.Format, timestampStrI)
		timestampJ, errJ := time.Parse(config.HRConfig.WalConfig.Format, timestampStrJ)
		if errI != nil || errJ != nil {
			return false
		}
		return timestampI.Before(timestampJ)
	})

	for _, file := range walFiles {
		filePath := filepath.Join(w.logDir, file.Name())

		slog.Debug("loading WAL", slog.Any("file", filePath))

		db, err := sql.Open("sqlite3", filePath)
		if err != nil {
			return fmt.Errorf("failed to open WAL file %s: %v", file.Name(), err)
		}

		rows, err := db.Query("SELECT event_type, payload, subscription_id, status_code, error, created_at FROM wal_event_deliveries")
		if err != nil {
			db.Close()
			return fmt.Errorf("failed to query WAL file %s: %v", file.Name(), err)
		}

		batch := make([]*event.EventDelivery, 0, batchSize)
		for rows.Next() {
			var e event.EventDelivery

			if err := rows.Scan(&e.EventType, &e.Payload, &e.SubscriptionId, &e.StatusCode, &e.Error, &e.CompletedAt); err != nil {
				slog.Error("failed to scan WAL file", slog.Any("file", file.Name()), slog.Any("error", err))
				continue
			}

			batch = append(batch, &e)

			// Process batch if it reaches the batch size
			if len(batch) >= batchSize {
				if err := f(batch); err != nil {
					rows.Close()
					db.Close()
					return err
				}
				batch = batch[:0] // Reset batch
			}
		}

		// Process any remaining events in the batch
		if len(batch) > 0 {
			if err := f(batch); err != nil {
				rows.Close()
				db.Close()
				return err
			}
		}

		if err := rows.Err(); err != nil {
			rows.Close()
			db.Close()
			return fmt.Errorf("failed to iterate WAL file %s: %v", file.Name(), err)
		}

		rows.Close()
		db.Close()
		// Remove the processed file
		os.Remove(filePath)
	}

	return nil
}

func (w *WALSQLite) DoAccounting(e []*event.EventDelivery) error {
	if w.accounting == nil {
		slog.Warn("accounting not initialized, skipping...")
		return errors.New("accounting not initialized")
	}
	w.accounting.CreateDeliveries(e)
	return nil
}

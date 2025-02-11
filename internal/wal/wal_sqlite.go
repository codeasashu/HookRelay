package wal

import (
	"database/sql"
	"encoding/json"
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
	logDir string
	curDB  *sql.DB
	mu     sync.Mutex
}

func NewSQLiteWAL() *WALSQLite {
	return &WALSQLite{
		logDir: config.HRConfig.WalConfig.Path,
	}
}

func (w *WALSQLite) Init(t time.Time) error {
	slog.Debug("initializing WAL at", slog.Any("log-dir", w.logDir))
	if err := os.MkdirAll(w.logDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	timestamp := t.Format("20060102")
	path := filepath.Join(w.logDir, fmt.Sprintf("wal_%s.sqlite3", timestamp))

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return err
	}

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
        event_id TEXT NOT NULL,
        owner_id TEXT NOT NULL,
        subscription_id TEXT NOT NULL,
        status_code INTEGER,
        error TEXT,
        latency REAL NOT NULL DEFAULT 0.0,
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

	if res, err := w.curDB.Exec("INSERT INTO wal_event_deliveries (event_id, owner_id, subscription_id, status_code, error, latency) VALUES (?,?,?,?,?,?)", e.EventID, e.OwnerId, e.SubscriptionId, e.StatusCode, e.Error, e.Latency); err != nil {
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

func (w *WALSQLite) Close() error {
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
		timestampI, errI := time.Parse("20060102_1504", timestampStrI)
		timestampJ, errJ := time.Parse("20060102_1504", timestampStrJ)
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

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
	logDir      string
	curDB       *sql.DB
	curFilename string
	mu          sync.Mutex
}

func NewSQLiteWAL() *WALSQLite {
	return &WALSQLite{
		logDir: config.HRConfig.WalConfig.Path,
	}
}

// genFile generates the filename (containing full path) for a WAL file
// filename format: wal_YYYY-MM-DD_HH-MM.sqlite3
func genFileName(t *time.Time, s string) string {
	if s != "" {
		return fmt.Sprintf("wal_%s_%s.sqlite3", t.Format(config.HRConfig.WalConfig.Format), s)
	}
	return fmt.Sprintf("wal_%s.sqlite3", t.Format(config.HRConfig.WalConfig.Format))
}

func getFileTimestamp(filename string) (*time.Time, error) {
	walFileLen := len(config.HRConfig.WalConfig.Format) + 4
	ftime, err := time.Parse(config.HRConfig.WalConfig.Format, filename[4:walFileLen])
	if err != nil {
		return nil, err
	}
	return &ftime, nil
}

func (w *WALSQLite) rotateFile() error {
	if w.curFilename == "" {
		return nil
	}
	_t, err := getFileTimestamp(w.curFilename)
	if err != nil {
		return err
	}

	oldPath := filepath.Join(w.logDir, w.curFilename)
	newPath := filepath.Join(w.logDir, genFileName(_t, "rotated"))
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}
	return nil
}

func (w *WALSQLite) Init(t time.Time) error {
	slog.Debug("initializing WAL at", slog.Any("log-dir", w.logDir))
	if err := os.MkdirAll(w.logDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	filename := genFileName(&t, "")
	path := filepath.Join(w.logDir, filename)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return err
	}

	// @TODO: use binary/JSONB for payload (see: https://sqlite.org/draft/jsonb.html)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
        owner_id TEXT NOT NULL,
        event_type TEXT NOT NULL,
        payload TEXT, -- SQLite does not have a native JSON type, store as TEXT
        idempotency_key TEXT);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS event_deliveries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
        event_type TEXT NOT NULL,
        payload TEXT NOT NULL,
        subscription_id TEXT NOT NULL,
        status_code INTEGER,
        error TEXT,
        start_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        complete_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)
	if err != nil {
		return err
	}
	w.curDB = db
	w.curFilename = filename
	return nil
}

func (w *WALSQLite) LogEvent(e *event.Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	payloadBytes, err := json.Marshal(e.Payload)
	if err != nil {
		payloadBytes = []byte("{}")
	}

	if res, err := w.curDB.Exec("INSERT INTO events (owner_id, event_type, payload, idempotency_key) VALUES (?,?,?,?)", e.OwnerId, e.EventType, payloadBytes, e.IdempotencyKey); err != nil {
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

	if res, err := w.curDB.Exec(
		"INSERT INTO event_deliveries (event_type, payload, subscription_id, status_code, error, start_at, complete_at) VALUES (?,?,?,?,?,?,?)",
		e.EventType, payloadBytes, e.SubscriptionId, e.StatusCode, e.Error, e.StartAt, e.CompleteAt,
	); err != nil {
		slog.Error("failed to log event delivery in WAL", slog.Any("error", err))
		return err
	} else {
		id, err := res.LastInsertId()
		if err == nil {
			e.ID = id
		} else {
			e.ID = -1
		}
		slog.Debug("logged event delivery in WAL", slog.Any("id", e.ID))
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

	stmt, err := tx.Prepare("INSERT INTO event_deliveries (event_type, payload, subscription_id, status_code, error, start_at, complete_at) VALUES (?,?,?,?,?,?,?)")
	if err != nil {
		slog.Error("failed to prepare statement for batch event logging", slog.Any("error", err))
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, e := range events {
		_, err := stmt.Exec(e.EventType, e.Payload, e.SubscriptionId, e.StatusCode, e.Error, e.StartAt, e.CompleteAt)
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
	// mu.Lock()
	// defer mu.Unlock()

	err := w.curDB.Close()
	if err != nil {
		return err
	}

	err = w.rotateFile()
	if err != nil {
		slog.Warn("failed to rename WAL file", slog.Any("err", err), slog.Any("filename", w.curFilename))
		return err
	}
	return nil
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
			if err := rows.Scan(&e.UID, &e.OwnerId, &e.EventType, &payload, &e.IdempotencyKey); err != nil {
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
		if file.IsDir() {
			continue
		}
		if matched, err := filepath.Match("*_rotated.sqlite3", file.Name()); err == nil && matched {
			walFiles = append(walFiles, file)
		}
	}

	if len(walFiles) == 0 {
		return fmt.Errorf("no valid WAL files found in log directory")
	}

	walFileLen := len(config.HRConfig.WalConfig.Format) + 4 // include prefix in overall len

	// Sort files by timestamp in ascending order
	sort.Slice(walFiles, func(i, j int) bool {
		timestampStrI := walFiles[i].Name()[4:walFileLen]
		timestampStrJ := walFiles[j].Name()[4:walFileLen]
		// Skip the currently processing files
		timestampI, errI := time.Parse(config.HRConfig.WalConfig.Format, timestampStrI)
		timestampJ, errJ := time.Parse(config.HRConfig.WalConfig.Format, timestampStrJ)
		if errI != nil || errJ != nil {
			return false
		}
		return timestampI.Before(timestampJ)
	})

	currTime := time.Now().Format(config.HRConfig.WalConfig.Format)
	slog.Info("picking WAL files < ", slog.Any("time", currTime))
	// Remove most recent entry from walFiles if walFiles[0] is the current time
	if walFiles[0].Name()[4:walFileLen] == currTime {
		walFiles = walFiles[1:]
	}

	for _, file := range walFiles {
		filePath := filepath.Join(w.logDir, file.Name())

		slog.Debug("loading WAL", slog.Any("file", filePath))

		db, err := sql.Open("sqlite3", filePath)
		if err != nil {
			return fmt.Errorf("failed to open WAL file %s: %v", file.Name(), err)
		}

		// Log the total number of event deliveries in the current WAL file
		var totalDeliveries int
		err = db.QueryRow("SELECT COUNT(*) FROM event_deliveries").Scan(&totalDeliveries)
		if err != nil {
			db.Close()
			return fmt.Errorf("failed to count event deliveries in WAL file %s: %v", file.Name(), err)
		}
		slog.Info("total event deliveries in WAL file", slog.String("file", file.Name()), slog.Int("count", totalDeliveries))

		rows, err := db.Query("SELECT event_type, payload, subscription_id, status_code, error, start_at, complete_at FROM event_deliveries")
		if err != nil {
			db.Close()
			return fmt.Errorf("failed to query WAL file %s: %v", file.Name(), err)
		}

		batch := make([]*event.EventDelivery, 0, batchSize)
		processedDeliveries := 0
		for rows.Next() {
			var e event.EventDelivery

			if err := rows.Scan(&e.EventType, &e.Payload, &e.SubscriptionId, &e.StatusCode, &e.Error, &e.StartAt, &e.CompleteAt); err != nil {
				slog.Error("failed to scan WAL file", slog.Any("file", file.Name()), slog.Any("error", err))
				continue
			}

			batch = append(batch, &e)
			processedDeliveries++

			// Process batch if it reaches the batch size
			if len(batch) >= batchSize {
				slog.Debug("processing batch", slog.Int("batch_size", len(batch)), slog.Int("processed", processedDeliveries))
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
			slog.Debug("processing final batch", slog.Int("batch_size", len(batch)), slog.Int("processed", processedDeliveries))
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

		slog.Info("processed event deliveries", slog.String("file", file.Name()), slog.Int("processed", processedDeliveries), slog.Int("total", totalDeliveries))

		// Remove the processed file
		if err := os.Remove(filePath); err != nil {
			slog.Error("failed to remove WAL file", slog.String("file", file.Name()), slog.Any("error", err))
		} else {
			slog.Info("removed WAL file", slog.String("file", file.Name()))
		}
	}

	return nil
}

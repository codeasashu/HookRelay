package wal

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	sync "sync"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
	_ "github.com/mattn/go-sqlite3"
)

type WALSQLite struct {
	cfg         *config.WALConfig
	logDir      string
	curDB       *sql.DB
	curFilename string
	mu          *sync.Mutex
}

func NewSQLiteWAL(cfg *config.WALConfig) *WALSQLite {
	return &WALSQLite{
		cfg:    cfg,
		logDir: cfg.Path,
		mu:     &sync.Mutex{},
	}
}

func (w *WALSQLite) GetMutex() *sync.Mutex {
	return w.mu
}

func (w *WALSQLite) GetDB() *sql.DB {
	return w.curDB
}

// genFile generates the filename (containing full path) for a WAL file
// filename format: wal_YYYY-MM-DD_HH-MM.sqlite3
func genFileName(t string, s string) string {
	if s != "" {
		return fmt.Sprintf("wal_%s_%s.sqlite3", t, s)
	}
	return fmt.Sprintf("wal_%s.sqlite3", t)
}

func getFileTimestamp(filename string, format string) (*time.Time, error) {
	walFileLen := len(format) + 4
	ftime, err := time.Parse(format, filename[4:walFileLen])
	if err != nil {
		return nil, err
	}
	return &ftime, nil
}

func (w *WALSQLite) rotateFile() error {
	if w.curFilename == "" {
		return nil
	}
	_t, err := getFileTimestamp(w.curFilename, w.cfg.Format)
	if err != nil {
		return err
	}

	oldPath := filepath.Join(w.logDir, w.curFilename)
	newPath := filepath.Join(w.logDir, genFileName(_t.Format(w.cfg.Format), "rotated"))
	slog.Info("rotateFile", "old", oldPath, "new", newPath)
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

	filename := genFileName(t.Format(w.cfg.Format), "")
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
        owner_id TEXT NOT NULL,
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

func (w *WALSQLite) Log(callback func(db *sql.DB) error) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return callback(w.curDB)
}

func (w *WALSQLite) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

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

//	func (w *WALSQLite) ForEachEvent(f func(c event.Event) error) error {
//		files, err := os.ReadDir(w.logDir)
//		if err != nil {
//			return fmt.Errorf("failed to read log directory: %v", err)
//		}
//
//		var walFiles []os.DirEntry
//
//		for _, file := range files {
//			if !file.IsDir() && filepath.Ext(file.Name()) == ".sqlite3" {
//				walFiles = append(walFiles, file)
//			}
//		}
//
//		if len(walFiles) == 0 {
//			return fmt.Errorf("no valid WAL files found in log directory")
//		}
//
//		// Sort files by timestamp in ascending order
//		sort.Slice(walFiles, func(i, j int) bool {
//			timestampStrI := walFiles[i].Name()[4:17]
//			timestampStrJ := walFiles[j].Name()[4:17]
//			timestampI, errI := time.Parse(config.HRConfig.WalConfig.Format, timestampStrI)
//			timestampJ, errJ := time.Parse(config.HRConfig.WalConfig.Format, timestampStrJ)
//			if errI != nil || errJ != nil {
//				return false
//			}
//			return timestampI.Before(timestampJ)
//		})
//
//		for _, file := range walFiles {
//			filePath := filepath.Join(w.logDir, file.Name())
//
//			slog.Debug("loading WAL", slog.Any("file", filePath))
//
//			db, err := sql.Open("sqlite3", filePath)
//			if err != nil {
//				return fmt.Errorf("failed to open WAL file %s: %v", file.Name(), err)
//			}
//
//			rows, err := db.Query("SELECT command FROM wal")
//			if err != nil {
//				return fmt.Errorf("failed to query WAL file %s: %v", file.Name(), err)
//			}
//
//			for rows.Next() {
//				var e event.Event
//				var payload string
//				// if err := rows.Scan(&command); err != nil {
//				if err := rows.Scan(&e.UID, &e.OwnerId, &e.EventType, &payload, &e.IdempotencyKey); err != nil {
//					return fmt.Errorf("failed to scan WAL file %s: %v", file.Name(), err)
//				}
//
//				if err := json.Unmarshal([]byte(payload), &e.Payload); err != nil {
//					slog.Error("failed to unmarshal payload", slog.Any("error", err))
//					continue
//				}
//				if err := f(e); err != nil {
//					return err
//				}
//			}
//
//			if err := rows.Err(); err != nil {
//				return fmt.Errorf("failed to iterate WAL file %s: %v", file.Name(), err)
//			}
//
//			if err := db.Close(); err != nil {
//				return fmt.Errorf("failed to close WAL file %s: %v", file.Name(), err)
//			}
//		}
//
//		return nil
//	}
func (w *WALSQLite) ForEachRotation(callbacks []func(db *sql.DB) error) error {
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

	walFileLen := len(w.cfg.Format) + 4 // include prefix in overall len

	// Sort files by timestamp in ascending order
	sort.Slice(walFiles, func(i, j int) bool {
		timestampStrI := walFiles[i].Name()[4:walFileLen]
		timestampStrJ := walFiles[j].Name()[4:walFileLen]
		// Skip the currently processing files
		timestampI, errI := time.Parse(w.cfg.Format, timestampStrI)
		timestampJ, errJ := time.Parse(w.cfg.Format, timestampStrJ)
		if errI != nil || errJ != nil {
			return false
		}
		return timestampI.Before(timestampJ)
	})

	currTime := time.Now().Format(w.cfg.Format)
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

		var wg sync.WaitGroup
		for _, cb := range callbacks {
			wg.Add(1)
			go func() {
				defer wg.Done()
				cb(db)
			}()
		}
		// Wait for all callbacks to finish, asyncronously
		// @TODO: Use timeout context to cancel callback execution after certain timeout
		wg.Wait()

		db.Close()

		slog.Info("processed rotation", slog.String("file", file.Name()))

		// Remove the processed file
		if err := os.Remove(filePath); err != nil {
			slog.Error("failed to remove WAL file", slog.String("file", file.Name()), slog.Any("error", err))
		} else {
			slog.Info("removed WAL file", slog.String("file", file.Name()))
		}
	}

	return nil
}

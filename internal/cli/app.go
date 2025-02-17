package cli

import (
	"context"
	"embed"
	"log/slog"
	"strings"
	"sync"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/database"
)

var F embed.FS

var (
	app  *App
	once sync.Once
)

type App struct {
	Ctx      context.Context
	Version  string
	DB       database.Database
	Logger   *slog.Logger
	Config   *config.Config
	IsWorker bool
}

func NewApp() *App {
	return &App{
		Version: GetVersion(),
	}
}

func GetAppInstance() *App {
	once.Do(func() {
		app = NewApp()
	})
	return app
}

func readVersion(fs embed.FS) ([]byte, error) {
	data, err := fs.ReadFile("VERSION")
	if err != nil {
		return nil, err
	}

	return data, nil
}

func GetVersion() string {
	v := "0.1.0"

	f, err := readVersion(F)
	if err != nil {
		return v
	}

	v = strings.TrimSpace(string(f))
	return v
}

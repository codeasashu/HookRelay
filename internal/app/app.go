package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/database"
	"github.com/codeasashu/HookRelay/internal/logger"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/wal"
	"github.com/gin-gonic/gin"
)

type HookRelayApp struct {
	Ctx     context.Context
	Cfg     *config.Config
	Metrics *metrics.Metrics

	Router     *gin.Engine
	HttpServer *http.Server
	Logger     *slog.Logger

	SubscriptionDb database.Database
	DeliveryDb     database.Database
	WAL            wal.AbstractWAL
	HttpClient     *http.Client
}

func loadCfg(cfgFile string) (*config.Config, error) {
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func FromConfig(cfgFile string, ctx context.Context, skipWal bool) (*HookRelayApp, error) {
	cfg, err := loadCfg(cfgFile)
	if err != nil {
		return nil, err
	}
	return NewApp(cfg, ctx, skipWal)
}

func NewApp(cfg *config.Config, ctx context.Context, skipWal bool) (*HookRelayApp, error) {
	app := &HookRelayApp{Cfg: cfg, Ctx: ctx}
	app.HttpClient = httpClientFunc(app)
	app.Router = newRouter(app)
	app.HttpServer = newServer(app)
	app.Metrics = initMetrics(app)
	app.Logger = initLogger(app)

	if !skipWal {
		wl, err := initWAL(app)
		if err != nil {
			return nil, err
		}
		app.WAL = wl
	}

	slog.SetDefault(app.Logger) // Set global logger as well
	return app, nil
}

func initWAL(a *HookRelayApp) (wal.AbstractWAL, error) {
	wl := wal.NewSQLiteWAL(&a.Cfg.WalConfig)
	if err := wl.Init(time.Now()); err != nil {
		slog.Error("could not initialize WAL", slog.Any("error", err))
		return nil, err
	}
	return wl, nil
}

func initLogger(a *HookRelayApp) *slog.Logger {
	logger := logger.New(a.Cfg.Logging)
	return logger
}

func (a *HookRelayApp) InitSubscriptionDb() error {
	db, err := subscriptionDb(a)
	a.SubscriptionDb = db
	return err
}

func (a *HookRelayApp) InitDeliveryDb() error {
	db, err := deliveryDb(a)
	a.DeliveryDb = db
	return err
}

func initMetrics(f *HookRelayApp) *metrics.Metrics {
	mt := metrics.NewMetrics(f.Cfg)
	mt.MetricsMiddleware(f.Router)
	return mt
}

func httpClientFunc(f *HookRelayApp) *http.Client {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	return client
}

func newRouter(f *HookRelayApp) *gin.Engine {
	router := gin.Default()
	return router
}

func newServer(f *HookRelayApp) *http.Server {
	s := &http.Server{
		Addr:         f.Cfg.Api.Addr,
		Handler:      f.Router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	return s
}

func subscriptionDb(f *HookRelayApp) (database.Database, error) {
	db, err := database.NewMySQLStorage(f.Cfg.Subscription.Database)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func deliveryDb(f *HookRelayApp) (database.Database, error) {
	db, err := database.NewMySQLStorage(f.Cfg.Delivery.Database)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (a *HookRelayApp) Start(errCh chan<- error) error {
	if err := a.HttpServer.ListenAndServe(); err != nil {
		errCh <- err
		return err
	}
	return nil
}

func (a *HookRelayApp) Shutdown(ctx context.Context) error {
	if err := a.HttpServer.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}

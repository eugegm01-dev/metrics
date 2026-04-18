// internal/app/server.go
package app

import (
	"context"
	"crypto/rsa"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/eugegm01-dev/metrics/internal/audit"
	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/crypto"
	"github.com/eugegm01-dev/metrics/internal/handler"
	"github.com/eugegm01-dev/metrics/internal/middleware"
	"github.com/eugegm01-dev/metrics/internal/repository"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

// Server encapsulates the HTTP server with graceful shutdown

type Server struct {
	httpServer *http.Server
	storage    repository.Storage
	audit      *audit.Subject
	logger     *zap.Logger
	db         *sql.DB
}

// RunServer запускает HTTP-сервер с конфигурацией из флагов и переменных окружения.
func RunServer() error {
	logger, err := zap.NewDevelopment(zap.AddStacktrace(zap.FatalLevel))
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	cfg, err := config.ParseServerConfig()
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	logger.Info("Server configuration",
		zap.String("address", cfg.Addr),
		zap.Duration("store_interval", cfg.StoreInterval),
		zap.String("file_storage_path", cfg.FileStoragePath),
		zap.Bool("restore", cfg.Restore),
		zap.String("database_dsn", cfg.DatabaseDSN),
	)

	// Initialize database
	var db *sql.DB
	if cfg.DatabaseDSN != "" {
		db, err = initDB(cfg, logger)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()
	}

	// Initialize storage
	storage, err := initStorage(cfg, db, logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	// Initialize audit
	var auditSubject *audit.Subject
	if cfg.AuditFile != "" || cfg.AuditURL != "" {
		auditSubject = audit.NewSubject()
		if cfg.AuditFile != "" {
			fileObs, err := audit.NewFileObserver(cfg.AuditFile)
			if err != nil {
				logger.Error("Failed to create file audit observer", zap.Error(err))
			} else {
				auditSubject.Attach(fileObs)
			}
		}
		if cfg.AuditURL != "" {
			httpObs := audit.NewHTTPObserver(cfg.AuditURL)
			auditSubject.Attach(httpObs)
		}
		defer func() {
			if auditSubject != nil {
				_ = auditSubject.Close()
			}
		}()
	}

	// Load crypto key
	var privKey *rsa.PrivateKey
	if cfg.CryptoKey != "" {
		k, err := crypto.LoadPrivateKey(cfg.CryptoKey)
		if err != nil {
			return fmt.Errorf("failed to load crypto key: %w", err)
		}
		privKey = k
		logger.Info("Server crypto key loaded")
	}

	// Initialize router
	r := initRouter(storage, logger, cfg.Key, auditSubject, privKey)

	// Create server
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Create server wrapper
	server := &Server{
		httpServer: srv,
		storage:    storage,
		audit:      auditSubject,
		logger:     logger,
		db:         db,
	}

	return server.runWithGracefulShutdown()
}

func (s *Server) runWithGracefulShutdown() error {
	// Channel to listen for errors from ListenAndServe
	serverErrors := make(chan error, 1)

	// Start server in goroutine
	go func() {
		s.logger.Info("Starting HTTP server", zap.String("address", s.httpServer.Addr))

		// Log registered routes
		if err := chi.Walk(s.httpServer.Handler.(*chi.Mux), func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
			s.logger.Debug("Registered route", zap.String("method", method), zap.String("route", route))
			return nil
		}); err != nil {
			s.logger.Warn("Failed to walk routes", zap.Error(err))
		}

		serverErrors <- s.httpServer.ListenAndServe()
	}()

	// Channel to listen for OS signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Block until we receive a signal or server error
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		s.logger.Info("Shutdown signal received",
			zap.String("signal", sig.String()),
			zap.String("description", getSignalDescription(sig)))

		// Start graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Step 1: Stop accepting new connections
		s.logger.Info("Stopping HTTP server - no longer accepting new connections")

		// Step 2: Graceful shutdown of HTTP server
		if err := s.httpServer.Shutdown(ctx); err != nil {
			if err == context.DeadlineExceeded {
				s.logger.Warn("HTTP shutdown timeout exceeded, forcing close")
			} else {
				s.logger.Error("HTTP server shutdown error", zap.Error(err))
			}
			// Force close if graceful shutdown fails
			if err := s.httpServer.Close(); err != nil {
				s.logger.Error("HTTP server force close error", zap.Error(err))
			}
		} else {
			s.logger.Info("HTTP server stopped gracefully")
		}

		// Step 3: Flush storage (for file-based storage)
		if err := s.flushStorage(ctx); err != nil {
			s.logger.Error("Failed to flush storage", zap.Error(err))
		}

		// Step 4: Close audit subject
		if s.audit != nil {
			s.logger.Info("Closing audit observers")
			if err := s.audit.Close(); err != nil {
				s.logger.Error("Audit close error", zap.Error(err))
			}
		}

		// Step 5: Close database connections
		if s.db != nil {
			s.logger.Info("Closing database connections")
			if err := s.db.Close(); err != nil {
				s.logger.Error("Database close error", zap.Error(err))
			}
		}

		s.logger.Info("Server shutdown completed successfully")
		return nil
	}
}

func (s *Server) flushStorage(ctx context.Context) error {
	// Try to flush file storage if supported
	if fs, ok := s.storage.(interface{ SaveToFile() error }); ok {
		s.logger.Info("Flushing metrics to file storage")
		if err := fs.SaveToFile(); err != nil {
			return fmt.Errorf("failed to save metrics to file: %w", err)
		}
		s.logger.Info("Metrics flushed to file storage successfully")
	}
	return nil
}

// getSignalDescription returns human-readable signal description
func getSignalDescription(sig os.Signal) string {
	switch sig {
	case syscall.SIGTERM:
		return "Termination signal (SIGTERM)"
	case syscall.SIGINT:
		return "Interrupt signal (SIGINT) - typically from Ctrl+C"
	case syscall.SIGQUIT:
		return "Quit signal (SIGQUIT) - typically from Ctrl+\\"
	default:
		return sig.String()
	}
}

func initDB(cfg *config.ServerConfig, logger *zap.Logger) (*sql.DB, error) {
	dsn := strings.Trim(cfg.DatabaseDSN, "'\"`")
	if dsn == "" {
		return nil, nil
	}
	if !strings.Contains(dsn, "://") {
		dsn = "postgres://postgres@" + dsn
	}

	logger.Info("Connecting to database", zap.String("dsn", dsn))

	var db *sql.DB
	var err error

	err = retry.Do(
		func() error {
			db, err = sql.Open("pgx", dsn)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return db.PingContext(ctx)
		},
		retry.Attempts(3),
		retry.DelayType(retry.BackOffDelay),
		retry.Delay(1*time.Second),
		retry.OnRetry(func(n uint, err error) {
			logger.Warn("Database connection attempt failed",
				zap.Uint("attempt", n+1),
				zap.Error(err))
		}),
	)

	if err != nil {
		if db != nil {
			db.Close()
		}
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	logger.Info("Database connected successfully")
	return db, nil
}

func initStorage(cfg *config.ServerConfig, db *sql.DB, logger *zap.Logger) (repository.Storage, error) {
	logger.Debug("Initializing storage",
		zap.String("file_path", cfg.FileStoragePath),
		zap.Duration("store_interval", cfg.StoreInterval),
		zap.Bool("restore", cfg.Restore),
		zap.Bool("has_db", db != nil))
	return repository.NewStorage(cfg.FileStoragePath, cfg.StoreInterval, cfg.Restore, db)
}

func initRouter(storage repository.Storage, logger *zap.Logger, key string, auditSubject *audit.Subject, privKey interface{}) *chi.Mux {
	r := chi.NewRouter()

	var rsaKey *rsa.PrivateKey
	if privKey != nil {
		var ok bool
		rsaKey, ok = privKey.(*rsa.PrivateKey)
		if !ok {
			logger.Error("privKey is not *rsa.PrivateKey")
		}
	}
	r.Use(middleware.CryptoMiddleware(rsaKey))

	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.GzipMiddleware)
	r.Use(middleware.LoggingMiddleware(logger))
	r.Use(middleware.HashMiddleware(key))

	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.Mount("/debug/pprof", http.HandlerFunc(pprof.Index))

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	r.Get("/", handler.IndexHTMLHandler(storage, key))
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler(storage, key))
	r.Get("/value/{type}/{name}", handler.GetMetricHandler(storage, key))
	r.Post("/update", handler.UpdateJSONHandler(storage, key))
	r.Post("/update/", handler.UpdateJSONHandler(storage, key))
	r.Post("/value", handler.ValueJSONHandler(storage, key))
	r.Post("/value/", handler.ValueJSONHandler(storage, key))

	r.Post("/updates", handler.UpdatesHandler(storage, key, auditSubject))
	r.Post("/updates/", handler.UpdatesHandler(storage, key, auditSubject))

	r.Post("/save", func(w http.ResponseWriter, r *http.Request) {
		if fs, ok := storage.(interface{ SaveToFile() error }); ok {
			if err := fs.SaveToFile(); err != nil {
				logger.Error("Failed to force save metrics to file", zap.Error(err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Metrics saved successfully\n"))
		} else {
			http.Error(w, "Storage does not support file saving", http.StatusBadRequest)
		}
	})

	return r
}

func runServer(srv *http.Server, logger *zap.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("Starting HTTP server", zap.String("address", srv.Addr))
		if err := chi.Walk(srv.Handler.(*chi.Mux), func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
			logger.Debug("Registered route", zap.String("method", method), zap.String("route", route))
			return nil
		}); err != nil {
			logger.Warn("Failed to walk routes", zap.Error(err))
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("Received shutdown signal, gracefully shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	logger.Info("Server stopped")
	return nil
}

// /Users/phil/dev/slop/internal/app/app.go
package appState

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/isaacphi/slop/internal/config"
)

// App holds the global application state
type App struct {
	Config *config.ConfigSchema
	Logger *slog.Logger
	closer io.Closer // For cleanup of resources like log files
}

var (
	globalApp *App
	initOnce  sync.Once
	initErr   error
	mu        sync.RWMutex
)

// Initialize creates the global app instance with the given overrides
func Initialize(overrides *config.RuntimeOverrides) error {
	initOnce.Do(func() {
		// Load base configuration first
		cfg, err := config.New(overrides)
		if err != nil {
			initErr = fmt.Errorf("failed to load config: %w", err)
			return
		}

		// Set up logger
		logger, closer, err := setupLogger(cfg.Log)
		if err != nil {
			initErr = fmt.Errorf("failed to setup logger: %w", err)
			return
		}

		mu.Lock()
		globalApp = &App{
			Config: cfg,
			Logger: logger,
			closer: closer,
		}
		mu.Unlock()

		// Set as default logger
		slog.SetDefault(logger)
	})
	return initErr
}

// Get returns the global app instance and panics if not initialized
func Get() *App {
	mu.RLock()
	defer mu.RUnlock()

	if globalApp == nil {
		panic("app not initialized")
	}
	return globalApp
}

// TryGet returns the global app instance and a boolean indicating if it's initialized
func TryGet() (*App, bool) {
	mu.RLock()
	defer mu.RUnlock()
	return globalApp, globalApp != nil
}

// Cleanup performs cleanup of app resources
func Cleanup() error {
	mu.Lock()
	defer mu.Unlock()

	if globalApp != nil && globalApp.closer != nil {
		return globalApp.closer.Close()
	}
	return nil
}

func setupLogger(cfg config.Log) (*slog.Logger, io.Closer, error) {
	var level slog.Level

	switch cfg.LogLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	if cfg.LogFile == "" {
		// Use stdout, no cleanup needed
		handler := slog.NewTextHandler(os.Stdout, opts)
		return slog.New(handler), nil, nil
	}

	// Create log file
	file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	handler := slog.NewTextHandler(file, opts)
	return slog.New(handler), file, nil
}

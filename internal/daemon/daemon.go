// Package daemon initializes and runs the recalld process.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/1tomany/recall/internal/database"
	"github.com/1tomany/recall/internal/httpapi"
)

// Run validates the listening port and data directory, initializes SQLite, and
// serves requests until ctx is canceled.
func Run(ctx context.Context, config Config, logger *slog.Logger) error {
	return run(ctx, config, logger, net.Listen)
}

type listenFunc func(network, address string) (net.Listener, error)

func run(ctx context.Context, config Config, logger *slog.Logger, listen listenFunc) error {
	listener, err := listen("tcp", net.JoinHostPort("", strconv.Itoa(config.Port)))
	if err != nil {
		return fmt.Errorf("bind to port %d: %w", config.Port, err)
	}
	defer listener.Close()

	if err := ensureWritableDirectory(config.DataDir); err != nil {
		return err
	}
	databasePath := database.DBPath(config.DataDir)
	db, err := database.Open(ctx, databasePath)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer db.Close()

	server := &http.Server{
		Handler:           httpapi.NewHandler(database.NewStore(db), logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}

	logger.Info("recalld started",
		"address", listener.Addr().String(),
		"database", databasePath,
	)
	serveError := make(chan error, 1)
	go func() {
		serveError <- server.Serve(listener)
	}()

	select {
	case err := <-serveError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		logger.Info("recalld shutting down")
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			_ = server.Close()
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		err := <-serveError
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}
}

func ensureWritableDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create data directory %q: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect data directory %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("data directory %q is not a directory", path)
	}

	probe, err := os.CreateTemp(path, ".recall-write-test-*")
	if err != nil {
		return fmt.Errorf("data directory %q is not writable: %w", path, err)
	}
	probePath := probe.Name()
	defer os.Remove(probePath)
	if err := probe.Close(); err != nil {
		return fmt.Errorf("verify write access to data directory %q: %w", path, err)
	}
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("clean up write probe in data directory %q: %w", path, err)
	}

	return nil
}

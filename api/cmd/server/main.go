// dns-manager-server is the HTTP server for managing DNS servers.
//
// Configuration is provided via environment variables:
//
//	LISTEN_ADDR   - server listen address (default ":8080")
//	RESOLVE_PATH  - path to resolv.conf (default "/etc/resolv.conf")
//	LOG_LEVEL     - logging level: debug, info, warn, error (default "info")
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	controllerhttp "github.com/ArtemNik1tin/dns-manager/api/internal/controller/http"
)

const (
	readTimeout       = 5 * time.Second
	readHeaderTimeout = 2 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Read configuration and initialise logger.
	lvl := parseLogLevel(os.Getenv("LOG_LEVEL"))
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))

	cfg := struct {
		addr        string
		resolvePath string
	}{
		addr:        getEnv("LISTEN_ADDR", ":8080"),
		resolvePath: getEnv("RESOLVE_PATH", "/etc/resolv.conf"),
	}

	// 2. Create a context that cancels on SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 3. Wire dependencies and build the HTTP handler.
	handler := controllerhttp.NewHandler(
		ctx,
		log,
		controllerhttp.Config{ResolvePath: cfg.resolvePath},
	)

	// 4. Configure the HTTP server with timeouts.
	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           handler,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}

	// 5. Start the server in a separate goroutine.
	serverError := make(chan error, 1)

	go func() {
		log.Info("server is starting", "addr", cfg.addr, "resolve_path", cfg.resolvePath)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- fmt.Errorf("listen and serve: %w", err)
		}
	}()

	// 6. Block until a fatal error or a shutdown signal.
	select {
	case err := <-serverError:
		return err
	case <-ctx.Done():
		log.Info("shutting down gracefully...")
	}

	// 7. Graceful shutdown with a timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Info("server stopped")

	return nil
}

// parseLogLevel converts a string level to slog.Level.
// Returns slog.LevelInfo for unknown values.
func parseLogLevel(level string) slog.Level {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return slog.LevelInfo
	}

	return lvl
}

// getEnv returns the value of the environment variable or a fallback.
func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}

	return fallback
}

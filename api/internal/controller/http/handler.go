package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/ArtemNik1tin/dns-manager/api/internal/adapter/resolver"
	"github.com/ArtemNik1tin/dns-manager/api/internal/usecase"
)

// Config holds configuration for the HTTP controller.
type Config struct {
	// ResolvePath is the path to resolv.conf.
	ResolvePath string
}

// NewHandler creates the HTTP handler, wiring adapters and use cases together.
func NewHandler(_ context.Context, log *slog.Logger, cfg Config) http.Handler {
	r := resolver.NewResolver(cfg.ResolvePath)
	uc := usecase.NewDNSUseCase(r)
	ctrl := NewDNSController(log, uc)

	mux := http.NewServeMux()
	mux.Handle("POST /api/dns", http.HandlerFunc(ctrl.Add))
	mux.Handle("DELETE /api/dns", http.HandlerFunc(ctrl.Delete))
	mux.Handle("GET /api/dns", http.HandlerFunc(ctrl.List))

	var handler http.Handler = mux
	handler = LoggingMiddleware(log, handler)

	return handler
}

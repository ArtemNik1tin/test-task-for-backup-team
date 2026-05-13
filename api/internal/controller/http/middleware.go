package http

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter is a wrapper around the standard http.ResponseWriter,
// which allows you to intercept the response status code for logging.
type responseWriter struct {
	http.ResponseWriter

	status      int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}

	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true
}

// LoggingMiddleware intercepts each request and writes information
// about the method, path, execution time, and final HTTP status to the log.
func LoggingMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := wrapResponseWriter(w)

		next.ServeHTTP(wrapped, r)

		log.Info("request processed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", time.Since(start),
			"ip", r.RemoteAddr,
		)
	})
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}
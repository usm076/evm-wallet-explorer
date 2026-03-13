package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
	"txn-info/config"
	"txn-info/handler"
	"txn-info/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "error", err)
		os.Exit(1)
	}

	svc := service.New(cfg)
	h := handler.New(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/transactions", h.GetTransactions)
	mux.HandleFunc("GET /api/balance", h.GetBalance)
	mux.Handle("GET /", http.FileServer(http.Dir("static")))

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("txn-info server started", "addr", addr)

	if err := http.ListenAndServe(addr, requestLogger(mux)); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

// requestLogger logs each incoming HTTP request and its response status + latency.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", rw.status,
			"latency_ms", time.Since(start).Milliseconds(),
		)
	})
}

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stanterprise/observer-mcp/internal/config"
	"github.com/stanterprise/observer-mcp/internal/db"
	"github.com/stanterprise/observer-mcp/internal/mcp"
	"github.com/stanterprise/observer-mcp/internal/tools"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{}))

	cfg, err := config.FromEnv()
	if err != nil {
		logger.Error("configuration error", "error", err)
		os.Exit(1)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pg, err := db.NewPostgresDB(cfg.PostgresDSN)
	if err != nil {
		logger.Error("postgres startup failed", "error", err)
		os.Exit(1)
	}
	sqlDB, err := pg.DB()
	if err != nil {
		logger.Error("postgres startup failed", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	mongoClient, err := db.NewMongoClient(rootCtx, cfg.MongoURI)
	if err != nil {
		logger.Error("mongo startup failed", "error", err)
		os.Exit(1)
	}
	if mongoClient != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = mongoClient.Disconnect(shutdownCtx)
		}()
	}

	healthServer := &http.Server{
		Addr: cfg.HealthAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}),
		ReadHeaderTimeout: 3 * time.Second,
	}

	go func() {
		if err := healthServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("health server failed", "error", err)
		}
	}()

	registry := tools.NewRegistry(pg, mongoClient, cfg.MongoDatabase, cfg.MongoCollection, cfg.ReadTimeout)
	server := mcp.NewServer(logger, registry.Tools())

	logger.Info("observer-mcp started", "health_addr", cfg.HealthAddr, "transport", cfg.TransportMode)

	if cfg.TransportMode == "http" {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})

		sseHandler := mcp.NewSSEHandler(server, logger, cfg.AuthToken)
		sseHandler.Register(mux)

		mcpHTTPServer := &http.Server{
			Addr:              cfg.MCPHTTPAddr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}

		go func() {
			<-rootCtx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = mcpHTTPServer.Shutdown(shutdownCtx)
		}()

		logger.Info("MCP HTTP/SSE listening", "addr", cfg.MCPHTTPAddr)
		if err := mcpHTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("mcp http server failed", "error", err)
			os.Exit(1)
		}
	} else {
		if err := server.Serve(rootCtx, os.Stdin, os.Stdout); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("mcp stdio server failed", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = healthServer.Shutdown(shutdownCtx)
}

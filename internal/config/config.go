package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	PostgresDSN     string
	MongoURI        string
	MongoDatabase   string
	MongoCollection string
	LogLevel        string
	HealthAddr      string
	ReadTimeout     time.Duration
	// Transport: "stdio" (default) or "http" (SSE for cloud/k8s deployment).
	TransportMode string
	// MCPHTTPAddr is the listen address when TransportMode is "http".
	MCPHTTPAddr string
	// AuthToken is an optional bearer token required by the SSE transport.
	// Leave empty to disable authentication.
	AuthToken string
}

func FromEnv() (Config, error) {
	cfg := Config{
		PostgresDSN:     firstNonEmpty(os.Getenv("POSTGRES_DSN"), os.Getenv("DATABASE_URL")),
		MongoURI:        firstNonEmpty(os.Getenv("MONGODB_URI"), os.Getenv("MONGO_URI")),
		MongoDatabase:   firstNonEmpty(os.Getenv("MCP_MONGO_DATABASE"), "observer"),
		MongoCollection: firstNonEmpty(os.Getenv("MCP_MONGO_COLLECTION"), "live_step_buffers"),
		LogLevel:        firstNonEmpty(os.Getenv("MCP_LOG_LEVEL"), "info"),
		HealthAddr:      firstNonEmpty(os.Getenv("MCP_HEALTH_ADDR"), ":9090"),
		TransportMode:   firstNonEmpty(os.Getenv("MCP_TRANSPORT"), "stdio"),
		MCPHTTPAddr:     firstNonEmpty(os.Getenv("MCP_HTTP_ADDR"), ":8080"),
		AuthToken:       os.Getenv("MCP_AUTH_TOKEN"),
	}

	if cfg.PostgresDSN == "" {
		return Config{}, errors.New("POSTGRES_DSN or DATABASE_URL is required")
	}

	readTimeoutSeconds, err := parseIntDefault("MCP_READ_TIMEOUT_SECONDS", 15)
	if err != nil {
		return Config{}, fmt.Errorf("invalid MCP_READ_TIMEOUT_SECONDS: %w", err)
	}
	cfg.ReadTimeout = time.Duration(readTimeoutSeconds) * time.Second

	switch cfg.TransportMode {
	case "stdio", "http":
	default:
		return Config{}, fmt.Errorf("invalid MCP_TRANSPORT %q (expected stdio or http)", cfg.TransportMode)
	}

	return cfg, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseIntDefault(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

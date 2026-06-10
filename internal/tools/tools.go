package tools

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/stanterprise/observer-mcp/internal/mcp"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

type Registry struct {
	pg              *gorm.DB
	mongo           *mongo.Client
	mongoDatabase   string
	mongoCollection string
	readTimeout     time.Duration
}

func NewRegistry(pg *gorm.DB, mongoClient *mongo.Client, mongoDatabase, mongoCollection string, readTimeout time.Duration) *Registry {
	return &Registry{
		pg:              pg,
		mongo:           mongoClient,
		mongoDatabase:   mongoDatabase,
		mongoCollection: mongoCollection,
		readTimeout:     readTimeout,
	}
}

func (r *Registry) Tools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "list_test_runs",
			Description: "List recent test runs from PostgreSQL with optional status filter.",
			InputSchema: objectSchema(map[string]any{
				"limit": numberSchema("Maximum number of runs to return (default 20, max 200)."),
				"status": map[string]any{
					"type":        "string",
					"description": "Optional run status filter, for example passed, failed, running.",
				},
			}),
			Handler: r.listTestRuns,
		},
		{
			Name:        "get_run_details",
			Description: "Get detailed run metadata plus aggregate run stats.",
			InputSchema: objectSchema(map[string]any{
				"run_id": map[string]any{"type": "string", "description": "Run identifier."},
			}),
			Handler: r.getRunDetails,
		},
		{
			Name:        "analyze_failure_patterns",
			Description: "Summarize top failure signatures from recent failed attempts.",
			InputSchema: objectSchema(map[string]any{
				"limit": numberSchema("Maximum number of failure signatures to return (default 10, max 100)."),
			}),
			Handler: r.analyzeFailurePatterns,
		},
		{
			Name:        "get_live_step_buffer",
			Description: "Read transient live step buffer entries from MongoDB collection live_step_buffers.",
			InputSchema: objectSchema(map[string]any{
				"run_id": map[string]any{"type": "string", "description": "Optional run ID filter."},
				"limit":  numberSchema("Maximum number of documents to return (default 20, max 100)."),
			}),
			Handler: r.getLiveStepBuffer,
		},
	}
}

func objectSchema(props map[string]any) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": props,
	}
}

func numberSchema(description string) map[string]any {
	return map[string]any{"type": "number", "description": description}
}

func (r *Registry) timeoutContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), r.readTimeout)
}

func intArg(args map[string]any, key string, fallback int, max int) (int, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return fallback, nil
	}
	switch typed := v.(type) {
	case float64:
		value := int(typed)
		if value <= 0 {
			return 0, fmt.Errorf("%s must be > 0", key)
		}
		if value > max {
			return max, nil
		}
		return value, nil
	case string:
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return 0, fmt.Errorf("%s must be numeric", key)
		}
		if parsed <= 0 {
			return 0, fmt.Errorf("%s must be > 0", key)
		}
		if parsed > max {
			return max, nil
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%s has unsupported type", key)
	}
}

func stringArg(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", nil
	}
	text, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return text, nil
}

func requireArg(args map[string]any, key string) (string, error) {
	value, err := stringArg(args, key)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", errors.New(key + " is required")
	}
	return value, nil
}

package tools

import (
	"fmt"
	"time"

	"github.com/stanterprise/observer-mcp/internal/mcp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (r *Registry) getLiveStepBuffer(_ mcp.CallContext, args map[string]any) (any, error) {
	if r.mongo == nil {
		return nil, fmt.Errorf("mongo connection is not available")
	}

	limit, err := intArg(args, "limit", 20, 100)
	if err != nil {
		return nil, err
	}
	runID, err := stringArg(args, "run_id")
	if err != nil {
		return nil, err
	}

	filter := bson.M{}
	if runID != "" {
		filter["run_id"] = runID
	}

	ctx, cancel := r.timeoutContext()
	defer cancel()

	coll := r.mongo.Database(r.mongoDatabase).Collection(r.mongoCollection)
	opts := options.Find().SetLimit(int64(limit)).SetSort(bson.D{{Key: "updated_at", Value: -1}, {Key: "created_at", Value: -1}})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("query live buffer: %w", err)
	}
	defer cursor.Close(ctx)

	docs := make([]bson.M, 0, limit)
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode live buffer doc: %w", err)
		}
		docs = append(docs, doc)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterate live buffer docs: %w", err)
	}

	return map[string]any{
		"database":   r.mongoDatabase,
		"collection": r.mongoCollection,
		"run_id":     runID,
		"count":      len(docs),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"documents":  docs,
	}, nil
}

package tools

import (
	"fmt"
	"time"

	"github.com/stanterprise/observer-mcp/internal/mcp"
	"gorm.io/gorm"
)

type runSummary struct {
	RunID      string     `json:"run_id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Duration   *int64     `json:"duration,omitempty"`
}

type runDetails struct {
	Run      runSummary       `json:"run"`
	RunStats map[string]int64 `json:"run_stats"`
}

type failurePattern struct {
	ErrorMessage string `json:"error_message"`
	Status       string `json:"status"`
	Count        int64  `json:"count"`
}

type failurePatternByTest struct {
	RunID        string `json:"run_id"`
	TestID       string `json:"test_id"`
	TestName     string `json:"test_name"`
	ErrorMessage string `json:"error_message"`
	Status       string `json:"status"`
	Count        int64  `json:"count"`
}

func (r *Registry) listTestRuns(_ mcp.CallContext, args map[string]any) (any, error) {
	if r.pg == nil {
		return nil, fmt.Errorf("postgres connection is not available")
	}

	limit, err := intArg(args, "limit", 20, 200)
	if err != nil {
		return nil, err
	}
	status, err := stringArg(args, "status")
	if err != nil {
		return nil, err
	}

	ctx, cancel := r.timeoutContext()
	defer cancel()

	results := make([]runSummary, 0, limit)
	q := r.pg.WithContext(ctx).
		Table("runs").
		Select("id as run_id, name, status, started_at, finished_at, duration").
		Order("started_at DESC NULLS LAST").
		Order("created_at DESC").
		Limit(limit)
	if status != "" {
		q = q.Where("UPPER(status) = UPPER(?)", status)
	}
	if err := q.Find(&results).Error; err != nil {
		return nil, fmt.Errorf("query runs: %w", err)
	}

	return map[string]any{"runs": results, "count": len(results)}, nil
}

func (r *Registry) getRunDetails(_ mcp.CallContext, args map[string]any) (any, error) {
	if r.pg == nil {
		return nil, fmt.Errorf("postgres connection is not available")
	}

	runID, err := requireArg(args, "run_id")
	if err != nil {
		return nil, err
	}

	ctx, cancel := r.timeoutContext()
	defer cancel()

	var out runDetails
	err = r.pg.WithContext(ctx).
		Table("runs").
		Select("id as run_id, name, status, started_at, finished_at, duration").
		Where("id = ?", runID).
		Take(&out.Run).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("run not found: %s", runID)
		}
		return nil, fmt.Errorf("query run: %w", err)
	}

	stats := map[string]int64{}
	var runStat struct {
		Total       int64
		Passed      int64
		Failed      int64
		Flaky       int64
		Skipped     int64
		Broken      int64
		Timedout    int64
		Interrupted int64
		Unknown     int64
		NotRun      int64 `gorm:"column:not_run"`
		Running     int64
	}
	err = r.pg.WithContext(ctx).
		Table("run_stats").
		Select("total, passed, failed, flaky, skipped, broken, timedout, interrupted, unknown, not_run, running").
		Where("run_id = ?", runID).
		Take(&runStat).Error
	if err == nil {
		stats["total"] = runStat.Total
		stats["passed"] = runStat.Passed
		stats["failed"] = runStat.Failed
		stats["flaky"] = runStat.Flaky
		stats["skipped"] = runStat.Skipped
		stats["broken"] = runStat.Broken
		stats["timedout"] = runStat.Timedout
		stats["interrupted"] = runStat.Interrupted
		stats["unknown"] = runStat.Unknown
		stats["not_run"] = runStat.NotRun
		stats["running"] = runStat.Running
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("query run stats: %w", err)
	}

	out.RunStats = stats
	return out, nil
}

func (r *Registry) analyzeFailurePatterns(_ mcp.CallContext, args map[string]any) (any, error) {
	if r.pg == nil {
		return nil, fmt.Errorf("postgres connection is not available")
	}

	limit, err := intArg(args, "limit", 10, 100)
	if err != nil {
		return nil, err
	}
	runID, err := stringArg(args, "run_id")
	if err != nil {
		return nil, err
	}

	ctx, cancel := r.timeoutContext()
	defer cancel()

	results := make([]failurePattern, 0, limit)
	q := r.pg.WithContext(ctx).
		Table("test_attempts").
		Select("COALESCE(NULLIF(error_message, ''), 'no_error_message') AS error_message, COALESCE(NULLIF(status, ''), 'unknown') AS status, COUNT(*)::bigint AS count").
		Where("UPPER(status) IN ?", []string{"FAILED", "BROKEN", "TIMEDOUT"})
	if runID != "" {
		q = q.Where("run_id = ?", runID)
	}
	err = q.Group("1, 2").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("query failure patterns: %w", err)
	}

	return map[string]any{"patterns": results, "count": len(results), "run_id": runID}, nil
}

func (r *Registry) analyzeFailurePatternsByTest(_ mcp.CallContext, args map[string]any) (any, error) {
	if r.pg == nil {
		return nil, fmt.Errorf("postgres connection is not available")
	}

	limit, err := intArg(args, "limit", 20, 200)
	if err != nil {
		return nil, err
	}
	runID, err := stringArg(args, "run_id")
	if err != nil {
		return nil, err
	}

	ctx, cancel := r.timeoutContext()
	defer cancel()

	results := make([]failurePatternByTest, 0, limit)
	q := r.pg.WithContext(ctx).
		Table("test_attempts ta").
		Select("ta.run_id, ta.test_id, COALESCE(NULLIF(t.name, ''), t.title, ta.test_id) AS test_name, COALESCE(NULLIF(ta.error_message, ''), 'no_error_message') AS error_message, COALESCE(NULLIF(ta.status, ''), 'unknown') AS status, COUNT(*)::bigint AS count").
		Joins("LEFT JOIN tests t ON t.id = ta.test_id AND t.run_id = ta.run_id").
		Where("UPPER(ta.status) IN ?", []string{"FAILED", "BROKEN", "TIMEDOUT"})
	if runID != "" {
		q = q.Where("ta.run_id = ?", runID)
	}
	err = q.Group("ta.run_id, ta.test_id, test_name, error_message, ta.status").
		Order("count DESC").
		Order("test_name ASC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("query failure patterns by test: %w", err)
	}

	return map[string]any{"patterns": results, "count": len(results), "run_id": runID}, nil
}

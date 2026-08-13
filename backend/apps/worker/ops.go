package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func runOpsLoop(ctx context.Context, db *pgxpool.Pool) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purgeOldInvocations(ctx, db)
			warnIfUnhealthy(ctx, db)
		}
	}
}

func purgeOldInvocations(ctx context.Context, db *pgxpool.Pool) {
	tag, err := db.Exec(ctx, `DELETE FROM model_invocations WHERE created_at < now() - interval '90 days'`)
	if err != nil {
		slog.Warn("model invocation retention failed", "error", err.Error())
		return
	}
	if tag.RowsAffected() > 0 {
		slog.Info("purged expired model invocations", "deleted", tag.RowsAffected())
	}
}

func warnIfUnhealthy(ctx context.Context, db *pgxpool.Pool) {
	var failed5m, stalePending, staleRunning int
	err := db.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status::text = 'failed' AND updated_at > now() - interval '5 minutes'),
			count(*) FILTER (WHERE status::text IN ('pending') AND created_at < now() - interval '5 minutes'),
			count(*) FILTER (WHERE status::text = 'running' AND COALESCE(started_at, created_at) < now() - interval '10 minutes')
		FROM async_tasks`,
	).Scan(&failed5m, &stalePending, &staleRunning)
	if err != nil {
		slog.Warn("queue health query failed", "error", err.Error())
		return
	}
	if failed5m >= 8 {
		slog.Warn("async task failure spike", "failed_5m", failed5m)
	}
	if stalePending >= 10 {
		slog.Warn("async task queue backlog", "stale_pending", stalePending)
	}
	if staleRunning >= 5 {
		slog.Warn("async tasks stuck running", "stale_running", staleRunning)
	}

	var failedCalls, totalCalls int
	err = db.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'failed'),
			count(*)
		FROM model_invocations
		WHERE created_at > now() - interval '5 minutes'`,
	).Scan(&failedCalls, &totalCalls)
	if err != nil {
		return
	}
	if totalCalls >= 10 && failedCalls*100/totalCalls >= 20 {
		slog.Warn("model error rate above threshold", "failed", failedCalls, "total", totalCalls)
	}
}

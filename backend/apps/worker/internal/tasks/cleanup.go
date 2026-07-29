package tasks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
)

func ObjectCleanupHandler(
	db *pgxpool.Pool,
	store *minio.Client,
	bucket string,
) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload sharedtasks.ObjectCleanupPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("decode cleanup task: %w", err)
		}
		_, _ = db.Exec(ctx, `
			UPDATE async_tasks
			SET status = 'running',
			    progress = 10,
			    started_at = COALESCE(started_at, now()),
			    updated_at = now()
			WHERE id = $1`,
			payload.TaskID,
		)
		if err := store.RemoveObject(
			ctx,
			bucket,
			payload.ObjectKey,
			minio.RemoveObjectOptions{},
		); err != nil {
			return failAsyncTask(
				ctx,
				db,
				payload.TaskID,
				"OBJECT_CLEANUP_FAILED",
				"文件清理暂时失败，将稍后重试",
				err,
			)
		}
		_, err := db.Exec(ctx, `
			UPDATE async_tasks
			SET status = 'succeeded',
			    progress = 100,
			    error_code = NULL,
			    error_summary = NULL,
			    error_message = NULL,
			    completed_at = now(),
			    updated_at = now()
			WHERE id = $1`,
			payload.TaskID,
		)
		return err
	}
}

func failAsyncTask(
	ctx context.Context,
	db *pgxpool.Pool,
	taskID, code, summary string,
	cause error,
) error {
	_, _ = db.Exec(ctx, `
		UPDATE async_tasks
		SET status = 'failed',
		    error_code = $2,
		    error_summary = $3,
		    error_message = $4,
		    completed_at = now(),
		    updated_at = now()
		WHERE id = $1`,
		taskID,
		code,
		summary,
		cause.Error(),
	)
	return cause
}

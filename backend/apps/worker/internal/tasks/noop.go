package tasks

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/logging"
)

const TypeNoop = "system:noop"

func HandleNoop(ctx context.Context, task *asynq.Task) error {
	logging.FromContext(ctx).Info("processed noop task", "task_type", task.Type())
	return nil
}

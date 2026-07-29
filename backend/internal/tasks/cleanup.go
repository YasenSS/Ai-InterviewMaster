package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const TypeObjectCleanup = "object:cleanup"

type ObjectCleanupPayload struct {
	TaskID    string `json:"task_id"`
	UserID    string `json:"user_id"`
	ObjectKey string `json:"object_key"`
}

func NewObjectCleanupTask(payload ObjectCleanupPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal cleanup task: %w", err)
	}
	return asynq.NewTask(TypeObjectCleanup, body), nil
}

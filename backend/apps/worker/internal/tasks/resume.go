package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const TypeResumeParse = "resume:parse"

type ResumeParsePayload struct {
	TaskID    string `json:"task_id"`
	ResumeID  string `json:"resume_id"`
	VersionID string `json:"version_id"`
	UserID    string `json:"user_id"`
}

func NewResumeParseTask(payload ResumeParsePayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal resume task: %w", err)
	}
	return asynq.NewTask(TypeResumeParse, body), nil
}

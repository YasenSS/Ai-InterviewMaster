package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const TypeQuestionGenerate = "question:generate"

type QuestionGeneratePayload struct {
	TaskID           string `json:"task_id"`
	QuestionSetID    string `json:"question_set_id"`
	UserID           string `json:"user_id"`
	ResumeID         string `json:"resume_id"`
	JobDescriptionID string `json:"job_description_id"`
	TargetRole       string `json:"target_role"`
}

func NewQuestionGenerateTask(payload QuestionGeneratePayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal question generate task: %w", err)
	}
	return asynq.NewTask(TypeQuestionGenerate, body), nil
}

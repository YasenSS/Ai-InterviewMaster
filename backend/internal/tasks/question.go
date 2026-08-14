package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const TypeQuestionGenerate = "question:generate"

type QuestionGeneratePayload struct {
	TaskID          string `json:"task_id"`
	QuestionSetID   string `json:"question_set_id"`
	SessionID       string `json:"session_id"`
	UserID          string `json:"user_id"`
	ResumeID        string `json:"resume_id"`
	ResumeVersionID string `json:"resume_version_id"`
	PrimaryLanguage string `json:"primary_language"`
	TargetCompany   string `json:"target_company"`
	TargetRole      string `json:"target_role"`
}

func NewQuestionGenerateTask(payload QuestionGeneratePayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal question generate task: %w", err)
	}
	return asynq.NewTask(TypeQuestionGenerate, body), nil
}

package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const (
	// TypeInterviewPrepare is the Agent V2 preparation task. The legacy name is
	// still registered by the worker so sessions created before v10 can finish.
	TypeInterviewPrepare = "interview:prepare"
	TypeQuestionGenerate = "question:generate"
)

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
	return newInterviewPrepareTask(TypeQuestionGenerate, payload)
}

func NewInterviewPrepareTask(payload QuestionGeneratePayload) (*asynq.Task, error) {
	return newInterviewPrepareTask(TypeInterviewPrepare, payload)
}

func newInterviewPrepareTask(taskType string, payload QuestionGeneratePayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal interview prepare task: %w", err)
	}
	return asynq.NewTask(taskType, body), nil
}

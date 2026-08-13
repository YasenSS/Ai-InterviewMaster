package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const TypeReportGenerate = "report:generate"

type ReportGeneratePayload struct {
	TaskID    string `json:"task_id"`
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	ReportID  string `json:"report_id"`
}

func NewReportGenerateTask(payload ReportGeneratePayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal report generate task: %w", err)
	}
	return asynq.NewTask(TypeReportGenerate, body), nil
}

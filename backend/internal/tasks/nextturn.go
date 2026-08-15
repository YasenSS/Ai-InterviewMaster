package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const TypeInterviewNextTurn = "interview:next_turn"

type InterviewNextTurnPayload struct {
	TaskID         string `json:"task_id"`
	DecisionID     string `json:"decision_id"`
	SessionID      string `json:"session_id"`
	AnsweredTurnID string `json:"answered_turn_id"`
	UserID         string `json:"user_id"`
	UseFallback    bool   `json:"use_fallback,omitempty"`
}

func NewInterviewNextTurnTask(payload InterviewNextTurnPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal interview next-turn task: %w", err)
	}
	return asynq.NewTask(TypeInterviewNextTurn, body), nil
}

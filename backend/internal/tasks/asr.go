package tasks

import (
	"encoding/json"
	"github.com/hibiken/asynq"
)

const TypeASRTranscribe = "asr:transcribe"

type ASRPayload struct {
	TaskID    string `json:"task_id"`
	UserID    string `json:"user_id"`
	ObjectKey string `json:"object_key"`
	Language  string `json:"language"`
}

func NewASRTask(payload ASRPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeASRTranscribe, data), nil
}

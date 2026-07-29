package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/hibiken/asynq"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
)

func ASRHandler(
	db *pgxpool.Pool,
	store *minio.Client,
	bucket, endpoint string,
) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload sharedtasks.ASRPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}
		fail := func(err error) error {
			return failAsyncTask(
				ctx,
				db,
				payload.TaskID,
				"ASR_TRANSCRIPTION_FAILED",
				"语音转写失败，请稍后重试",
				err,
			)
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
		object, err := store.GetObject(ctx, bucket, payload.ObjectKey, minio.GetObjectOptions{})
		if err != nil {
			return fail(err)
		}
		defer object.Close()
		audio, err := io.ReadAll(io.LimitReader(object, 50*1024*1024))
		if err != nil {
			return fail(err)
		}
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("audio_file", "interview-audio")
		if err != nil {
			return fail(err)
		}
		if _, err = part.Write(audio); err != nil {
			return fail(err)
		}
		if err := writer.Close(); err != nil {
			return fail(err)
		}
		endpointURL := strings.TrimRight(endpoint, "/") + "/asr?output=json"
		if payload.Language != "" {
			endpointURL += "&language=" + url.QueryEscape(payload.Language)
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, &body)
		if err != nil {
			return fail(err)
		}
		request.Header.Set("Content-Type", writer.FormDataContentType())
		response, err := (&http.Client{}).Do(request)
		if err != nil {
			return fail(fmt.Errorf("call ASR: %w", err))
		}
		defer response.Body.Close()
		if response.StatusCode/100 != 2 {
			return fail(fmt.Errorf("ASR returned %s", response.Status))
		}
		var result map[string]any
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			return fail(err)
		}
		encoded, _ := json.Marshal(result)
		_, err = db.Exec(ctx, `
			UPDATE async_tasks
			SET status = 'succeeded',
			    progress = 100,
			    result = $2,
			    error_code = NULL,
			    error_summary = NULL,
			    error_message = NULL,
			    completed_at = now(),
			    updated_at = now()
			WHERE id = $1`,
			payload.TaskID,
			encoded,
		)
		return err
	}
}

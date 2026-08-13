package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/internal/aiworkflow"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
)

func ResumeParseHandler(db *pgxpool.Pool, store *minio.Client, bucket, tikaURL string, chat platformai.ChatModel) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload sharedtasks.ResumeParsePayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("decode task: %w", err)
		}
		_, _ = db.Exec(ctx, `
			UPDATE async_tasks
			SET status='running', progress=10, started_at=COALESCE(started_at, now()), updated_at=now()
			WHERE id=$1`, payload.TaskID)
		_, _ = db.Exec(ctx, `UPDATE resumes SET status='processing',updated_at=now() WHERE id=$1`, payload.ResumeID)
		var key, contentType string
		if err := db.QueryRow(ctx, `SELECT object_key, content_type FROM resume_versions WHERE id=$1`, payload.VersionID).Scan(&key, &contentType); err != nil {
			return failResume(ctx, db, payload, err)
		}
		object, err := store.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
		if err != nil {
			return failResume(ctx, db, payload, err)
		}
		defer object.Close()
		data, err := io.ReadAll(io.LimitReader(object, 20*1024*1024))
		if err != nil {
			return failResume(ctx, db, payload, err)
		}
		text, err := extractText(ctx, data, contentType, tikaURL)
		if err != nil {
			return failResume(ctx, db, payload, err)
		}
		text, err = aiworkflow.PreprocessResumeText(text)
		if err != nil {
			return failResume(ctx, db, payload, err)
		}
		if _, err = db.Exec(ctx, `
			UPDATE resume_versions
			SET extracted_text=$2, processed_at=now(), parse_error=NULL
			WHERE id=$1`, payload.VersionID, text); err != nil {
			return failResume(ctx, db, payload, err)
		}
		_, _ = db.Exec(ctx, `UPDATE async_tasks SET progress=40, updated_at=now() WHERE id=$1`, payload.TaskID)

		var extraction contract.ResumeExtraction
		var extractorModel, promptVersion string
		if chat != nil {
			extracted, response, extractErr := aiworkflow.ExtractResume(ctx, chat, payload.UserID, payload.TaskID, payload.VersionID, text)
			if extractErr != nil {
				return failResume(ctx, db, payload, extractErr)
			}
			extraction = extracted
			extractorModel = response.Model
			promptVersion = aiworkflow.PromptV1
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `DELETE FROM resume_facts WHERE resume_version_id=$1`, payload.VersionID); err != nil {
			return failResume(ctx, db, payload, err)
		}
		if _, err = tx.Exec(ctx, `DELETE FROM resume_chunks WHERE resume_version_id=$1`, payload.VersionID); err != nil {
			return failResume(ctx, db, payload, err)
		}
		for _, fact := range extraction.Facts {
			value, _ := json.Marshal(fact.FactValue)
			if _, err = tx.Exec(ctx, `
				INSERT INTO resume_facts(resume_version_id,fact_type,fact_key,fact_value,source_excerpt,confidence)
				VALUES($1,$2,$3,$4,$5,$6)`,
				payload.VersionID, fact.FactType, fact.FactKey, value, fact.SourceExcerpt, fact.Confidence,
			); err != nil {
				return failResume(ctx, db, payload, err)
			}
		}
		for i, chunk := range chunks(text, 1000) {
			if _, err = tx.Exec(ctx, `
				INSERT INTO resume_chunks(resume_version_id,chunk_no,content,token_count,embedding,embedding_model)
				VALUES($1,$2,$3,$4,NULL,NULL)`,
				payload.VersionID, i+1, chunk, len(strings.Fields(chunk)),
			); err != nil {
				return failResume(ctx, db, payload, err)
			}
		}
		if _, err = tx.Exec(ctx, `
			UPDATE resume_versions
			SET extractor_model=$2, prompt_version=$3
			WHERE id=$1`, payload.VersionID, nullString(extractorModel), nullString(promptVersion)); err != nil {
			return failResume(ctx, db, payload, err)
		}
		if _, err = tx.Exec(ctx, `UPDATE resumes SET status='completed',updated_at=now() WHERE id=$1`, payload.ResumeID); err != nil {
			return failResume(ctx, db, payload, err)
		}
		if _, err = tx.Exec(ctx, `
			UPDATE async_tasks
			SET status='succeeded', progress=100, error_code=NULL, error_summary=NULL, error_message=NULL, completed_at=now(), updated_at=now()
			WHERE id=$1`, payload.TaskID); err != nil {
			return failResume(ctx, db, payload, err)
		}
		return tx.Commit(ctx)
	}
}

func failResume(ctx context.Context, db *pgxpool.Pool, p sharedtasks.ResumeParsePayload, err error) error {
	_, _ = db.Exec(ctx, `
		UPDATE async_tasks
		SET status='failed', error_code='RESUME_PARSE_FAILED', error_summary='简历解析失败，请检查文件后重试',
		    error_message=$2, completed_at=now(), updated_at=now()
		WHERE id=$1`, p.TaskID, err.Error())
	_, _ = db.Exec(ctx, `UPDATE resumes SET status='failed',updated_at=now() WHERE id=$1`, p.ResumeID)
	_, _ = db.Exec(ctx, `UPDATE resume_versions SET parse_error='简历解析失败，请检查文件后重试' WHERE id=$1`, p.VersionID)
	return err
}

func chunks(s string, n int) []string {
	r := []rune(s)
	var out []string
	for len(r) > 0 {
		end := n
		if len(r) < end {
			end = len(r)
		}
		out = append(out, string(r[:end]))
		r = r[end:]
	}
	return out
}

func extractText(ctx context.Context, data []byte, contentType, tikaURL string) (string, error) {
	if strings.HasPrefix(contentType, "text/") {
		return strings.TrimSpace(string(data)), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, strings.TrimRight(tikaURL, "/")+"/tika", strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", fmt.Errorf("call Tika: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Tika returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

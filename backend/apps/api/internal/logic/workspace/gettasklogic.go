// Task loading and retry share the same reference and error mapping.
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5"
	"github.com/zeromicro/go-zero/core/logx"
)

type taskScanner interface {
	Scan(dest ...any) error
}

const taskSelect = `
	SELECT task.id::text,
	       CASE
	           WHEN task.task_type = 'question.generate' THEN 'interview.prepare'
	           ELSE task.task_type
	       END,
	       task.status::text,
	       task.progress::int,
	       CASE
	           WHEN task.task_type = 'resume.parse' THEN 'resume_version'
	           WHEN task.task_type = 'object.cleanup' THEN 'resume_version'
	           WHEN task.task_type = 'asr.transcribe' THEN 'other'
	           WHEN task.task_type = 'question.generate' THEN 'interview_session'
	           WHEN task.task_type = 'report.generate' THEN 'interview_session'
	           ELSE 'other'
	       END,
	       task.ref_id::text,
	       COALESCE(session.title, resume.title, ''),
	       COALESCE(task.error_code, ''),
	       COALESCE(task.error_summary, ''),
	       COALESCE(task.result, 'null'::jsonb),
	       COALESCE(task.retry_of_task_id::text, ''),
	       task.created_at,
	       task.updated_at,
	       task.started_at,
	       task.completed_at
	FROM async_tasks AS task
	LEFT JOIN resume_versions AS version
	  ON task.task_type = 'resume.parse'
	 AND version.id = task.ref_id
	LEFT JOIN resumes AS resume ON resume.id = version.resume_id
	LEFT JOIN interview_sessions AS session
	  ON task.task_type = 'question.generate'
	 AND session.id = task.ref_id
`

func scanTask(row taskScanner) (types.TaskResponse, error) {
	var item types.TaskResponse
	var errorCode, errorSummary string
	var raw []byte
	var createdAt, updatedAt time.Time
	var startedAt, completedAt *time.Time
	err := row.Scan(
		&item.Id,
		&item.Type,
		&item.Status,
		&item.Progress,
		&item.Reference.Type,
		&item.Reference.Id,
		&item.Reference.Title,
		&errorCode,
		&errorSummary,
		&raw,
		&item.RetryOfTaskId,
		&createdAt,
		&updatedAt,
		&startedAt,
		&completedAt,
	)
	if err != nil {
		return item, err
	}
	if errorCode != "" || errorSummary != "" {
		item.Error = &types.TaskErrorResponse{Code: errorCode, Message: errorSummary}
	}
	if string(raw) != "null" {
		_ = json.Unmarshal(raw, &item.Result)
	}
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	item.StartedAt = formatOptionalTime(startedAt)
	item.CompletedAt = formatOptionalTime(completedAt)
	return item, nil
}

func loadTask(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, taskID string,
) (*types.TaskResponse, error) {
	item, err := scanTask(svcCtx.Database.QueryRow(ctx,
		taskSelect+` WHERE task.id = $1 AND task.user_id = $2`,
		taskID,
		userID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("TASK_NOT_FOUND", "未找到该任务", err)
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func retryTask(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, originalTaskID string,
) (*types.TaskAcceptedResponse, error) {
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var taskType, status, refID string
	var resultRaw []byte
	err = tx.QueryRow(ctx, `
		SELECT task_type, status::text, ref_id::text, COALESCE(result, 'null'::jsonb)
		FROM async_tasks AS task
		WHERE task.id = $1 AND task.user_id = $2
		  AND NOT EXISTS (
			SELECT 1
			FROM async_tasks AS newer
			WHERE newer.user_id = task.user_id
			  AND newer.ref_id = task.ref_id
			  AND newer.task_type = task.task_type
			  AND (newer.created_at, newer.id) > (task.created_at, task.id)
		  )
		FOR UPDATE`,
		originalTaskID,
		userID,
	).Scan(&taskType, &status, &refID, &resultRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("TASK_NOT_FOUND", "未找到该任务", err)
	}
	if err != nil {
		return nil, err
	}
	if status != "failed" || (taskType != "resume.parse" && taskType != "object.cleanup" && taskType != "question.generate" && taskType != "report.generate") {
		return nil, conflict("TASK_NOT_RETRYABLE", "该任务不支持重试", nil)
	}
	var existingID, existingStatus string
	err = tx.QueryRow(ctx, `
		SELECT id::text, status::text
		FROM async_tasks
		WHERE retry_of_task_id = $1
		  AND user_id = $2
		  AND status IN ('pending', 'running')
		ORDER BY created_at DESC, id DESC
		LIMIT 1`,
		originalTaskID,
		userID,
	).Scan(&existingID, &existingStatus)
	if err == nil {
		return &types.TaskAcceptedResponse{TaskId: existingID, Status: existingStatus}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	newTaskID := uuid.NewString()
	_, err = tx.Exec(ctx, `
		INSERT INTO async_tasks (
			id,
			user_id,
			task_type,
			ref_id,
			status,
			progress,
			result,
			retry_of_task_id
		)
		VALUES ($1, $2, $3, $4, 'pending', 0, $5, $6)`,
		newTaskID,
		userID,
		taskType,
		refID,
		resultRaw,
		originalTaskID,
	)
	if err != nil {
		return nil, err
	}
	var queuedTask *asynq.Task
	var queue string
	switch taskType {
	case "resume.parse":
		var resumeID string
		err = tx.QueryRow(ctx, `
			SELECT resume.id::text
			FROM resume_versions AS version
			JOIN resumes AS resume ON resume.id = version.resume_id
			WHERE version.id = $1 AND resume.user_id = $2 AND resume.status = 'failed'`,
			refID,
			userID,
		).Scan(&resumeID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, conflict("TASK_NOT_RETRYABLE", "简历版本已不存在，无法重试", nil)
		}
		if err != nil {
			return nil, err
		}
		queuedTask, err = sharedtasks.NewResumeParseTask(sharedtasks.ResumeParsePayload{
			TaskID:    newTaskID,
			ResumeID:  resumeID,
			VersionID: refID,
			UserID:    userID,
		})
		if err == nil {
			_, err = tx.Exec(ctx, `
				UPDATE resumes SET status = 'pending', updated_at = now() WHERE id = $1`,
				resumeID,
			)
		}
		queue = "heavy"
	case "object.cleanup":
		var result map[string]any
		_ = json.Unmarshal(resultRaw, &result)
		objectKey, _ := result["object_key"].(string)
		if objectKey == "" {
			return nil, conflict("TASK_NOT_RETRYABLE", "清理任务缺少对象信息", nil)
		}
		queuedTask, err = sharedtasks.NewObjectCleanupTask(sharedtasks.ObjectCleanupPayload{
			TaskID:    newTaskID,
			UserID:    userID,
			ObjectKey: objectKey,
		})
		queue = "default"
	case "question.generate":
		var sessionID, questionSetID, resumeID, resumeVersionID, primaryLanguage, targetCompany, targetRole string
		err = tx.QueryRow(ctx, `
			SELECT session.id::text, qset.id::text, session.resume_id::text, session.resume_version_id::text,
			       session.primary_language, session.target_company,
			       COALESCE(qset.target_role, 'backend_development')
			FROM interview_sessions AS session
			JOIN question_sets AS qset ON qset.id=session.question_set_id
			WHERE session.id=$1 AND session.user_id=$2
			  AND session.status='failed' AND qset.status='failed'`,
			refID, userID,
		).Scan(&sessionID, &questionSetID, &resumeID, &resumeVersionID, &primaryLanguage, &targetCompany, &targetRole)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, conflict("TASK_NOT_RETRYABLE", "面试当前状态不允许重试准备", nil)
		}
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE question_sets SET status='generating'::question_set_status, updated_at=now() WHERE id=$1`, questionSetID); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE interview_sessions SET status='preparing', updated_at=now() WHERE id=$1`, sessionID); err != nil {
			return nil, err
		}
		queuedTask, err = sharedtasks.NewQuestionGenerateTask(sharedtasks.QuestionGeneratePayload{
			TaskID: newTaskID, QuestionSetID: questionSetID, SessionID: sessionID,
			UserID: userID, ResumeID: resumeID, ResumeVersionID: resumeVersionID, PrimaryLanguage: primaryLanguage,
			TargetCompany: targetCompany, TargetRole: targetRole,
		})
		queue = "heavy"
	case "report.generate":
		var reportID string
		err = tx.QueryRow(ctx, `
			SELECT report.id::text
			FROM interview_reports AS report
			JOIN interview_sessions AS session ON session.id = report.session_id
			WHERE report.session_id=$1 AND report.status='failed' AND session.user_id=$2`,
			refID, userID,
		).Scan(&reportID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, conflict("TASK_NOT_RETRYABLE", "报告已不存在，无法重试", nil)
		}
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE interview_reports SET status='pending', error_code=NULL, error_summary=NULL, updated_at=now() WHERE id=$1`, reportID); err != nil {
			return nil, err
		}
		queuedTask, err = sharedtasks.NewReportGenerateTask(sharedtasks.ReportGeneratePayload{
			TaskID: newTaskID, SessionID: refID, UserID: userID, ReportID: reportID,
		})
		queue = "heavy"
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	_, err = svcCtx.TaskClient.EnqueueContext(ctx, queuedTask, asynq.Queue(queue))
	if err != nil {
		_, _ = svcCtx.Database.Exec(ctx, `
			UPDATE async_tasks
			SET status = 'failed',
			    error_code = 'TASK_ENQUEUE_FAILED',
			    error_summary = '任务暂时无法启动，请重试',
			    error_message = $2,
			    completed_at = now(),
			    updated_at = now()
			WHERE id = $1`,
			newTaskID,
			err.Error(),
		)
		switch taskType {
		case "question.generate":
			_, _ = svcCtx.Database.Exec(ctx, `
				UPDATE interview_sessions SET status='failed', updated_at=now()
				WHERE id=$1 AND user_id=$2 AND status='preparing'`, refID, userID)
			_, _ = svcCtx.Database.Exec(ctx, `
				UPDATE question_sets AS qset SET status='failed', updated_at=now()
				FROM interview_sessions AS session
				WHERE session.id=$1 AND session.user_id=$2
				  AND qset.id=session.question_set_id AND qset.status='generating'`, refID, userID)
		case "report.generate":
			_, _ = svcCtx.Database.Exec(ctx, `
				UPDATE interview_reports AS report
				SET status='failed', error_code='TASK_ENQUEUE_FAILED',
				    error_summary='报告任务暂时无法启动', updated_at=now()
				FROM interview_sessions AS session
				WHERE report.session_id=$1 AND session.id=report.session_id
				  AND session.user_id=$2 AND report.status='pending'`, refID, userID)
		}
		return nil, err
	}
	return &types.TaskAcceptedResponse{TaskId: newTaskID, Status: "pending"}, nil
}

type GetTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTaskLogic {
	return &GetTaskLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetTaskLogic) GetTask(req *types.TaskPath) (*types.TaskResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return loadTask(l.ctx, l.svcCtx, userID, req.Id)
}

type RetryTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRetryTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetryTaskLogic {
	return &RetryTaskLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RetryTaskLogic) RetryTask(req *types.TaskPath) (*types.TaskAcceptedResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return retryTask(l.ctx, l.svcCtx, userID, req.Id)
}

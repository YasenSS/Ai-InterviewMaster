package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5"
)

func ensureReportGeneration(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
) (string, string, error) {
	var reportID, status string
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT id::text, status FROM interview_reports WHERE session_id=$1`, sessionID,
	).Scan(&reportID, &status)
	if err == nil {
		if status == "completed" || status == "degraded" || status == "failed" {
			return reportID, status, nil
		}
		return enqueueOrReuseReportTask(ctx, svcCtx, userID, sessionID, reportID, status)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", err
	}

	reportID = uuid.NewString()
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO interview_reports (id, session_id, overall_score, status)
		VALUES ($1,$2,0,'pending')
		ON CONFLICT (session_id) DO NOTHING`, reportID, sessionID)
	if err != nil {
		return "", "", err
	}
	if err := tx.QueryRow(ctx, `SELECT id::text, status FROM interview_reports WHERE session_id=$1`, sessionID).Scan(&reportID, &status); err != nil {
		return "", "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", err
	}
	if status == "completed" || status == "degraded" {
		return reportID, status, nil
	}
	return enqueueOrReuseReportTask(ctx, svcCtx, userID, sessionID, reportID, status)
}

func enqueueOrReuseReportTask(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID, reportID, reportStatus string,
) (string, string, error) {
	var taskID, taskStatus string
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT id::text, status::text
		FROM async_tasks
		WHERE user_id=$1 AND ref_id=$2 AND task_type='report.generate'
		ORDER BY created_at DESC LIMIT 1`,
		userID, sessionID,
	).Scan(&taskID, &taskStatus)
	if err == nil {
		if taskStatus == "pending" || taskStatus == "running" || taskStatus == "succeeded" {
			if taskStatus == "pending" {
				_ = enqueueReportTask(ctx, svcCtx, userID, sessionID, reportID, taskID)
			}
			return reportID, reportStatus, nil
		}
		if reportStatus == "failed" {
			return reportID, reportStatus, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", err
	}

	taskID = uuid.NewString()
	_, err = svcCtx.Database.Exec(ctx, `
		INSERT INTO async_tasks (id, user_id, task_type, ref_id, status, progress)
		VALUES ($1,$2,'report.generate',$3,'pending',0)`,
		taskID, userID, sessionID)
	if isUniqueViolation(err) {
		err = svcCtx.Database.QueryRow(ctx, `
			SELECT id::text, status::text
			FROM async_tasks
			WHERE user_id=$1 AND ref_id=$2 AND task_type='report.generate'
			  AND status IN ('pending', 'running')
			ORDER BY created_at DESC LIMIT 1`,
			userID, sessionID,
		).Scan(&taskID, &taskStatus)
		if err != nil {
			return "", "", err
		}
		return reportID, reportStatus, nil
	}
	if err != nil {
		return "", "", err
	}
	if err := enqueueReportTask(ctx, svcCtx, userID, sessionID, reportID, taskID); err != nil {
		return "", "", err
	}
	return reportID, "pending", nil
}

func enqueueReportTask(ctx context.Context, svcCtx *svc.ServiceContext, userID, sessionID, reportID, taskID string) error {
	queued, err := sharedtasks.NewReportGenerateTask(sharedtasks.ReportGeneratePayload{
		TaskID: taskID, SessionID: sessionID, UserID: userID, ReportID: reportID,
	})
	if err != nil {
		return err
	}
	if _, err := svcCtx.TaskClient.EnqueueContext(ctx, queued, asynq.Queue("heavy"), asynq.Unique(10*time.Minute)); err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
		_, _ = svcCtx.Database.Exec(ctx, `
			UPDATE interview_reports
			SET status='failed', error_code='TASK_ENQUEUE_FAILED', error_summary='报告任务暂时无法启动', updated_at=now()
			WHERE id=$1`, reportID)
		_, _ = svcCtx.Database.Exec(ctx, `
			UPDATE async_tasks
			SET status='failed', error_code='TASK_ENQUEUE_FAILED', error_summary='报告任务暂时无法启动',
			    error_message=$2, completed_at=now(), updated_at=now()
			WHERE id=$1`, taskID, err.Error())
		return apperror.Unavailable("报告生成任务暂时无法启动，请稍后重试", nil, err)
	}
	return nil
}

func retryReportGeneration(ctx context.Context, svcCtx *svc.ServiceContext, userID, sessionID string) (string, error) {
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var reportID, status string
	err = tx.QueryRow(ctx, `
		SELECT report.id::text,report.status
		FROM interview_reports report
		JOIN interview_sessions session ON session.id=report.session_id
		WHERE report.session_id=$1 AND session.user_id=$2 AND session.status='completed'
		FOR UPDATE OF report`, sessionID, userID,
	).Scan(&reportID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", resourceNotFound("INTERVIEW_REPORT_NOT_FOUND", "未找到该面试报告", err)
	}
	if err != nil {
		return "", err
	}
	if status != "failed" {
		return "", conflict("INTERVIEW_REPORT_NOT_FAILED", "只有生成失败的报告可以重试", nil)
	}
	taskID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		UPDATE interview_reports SET status='pending',error_code=NULL,error_summary=NULL,updated_at=now()
		WHERE id=$1`, reportID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO async_tasks (id,user_id,task_type,ref_id,status,progress)
		VALUES ($1,$2,'report.generate',$3,'pending',0)`, taskID, userID, sessionID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	if err := enqueueReportTask(ctx, svcCtx, userID, sessionID, reportID, taskID); err != nil {
		return "", err
	}
	return reportID, nil
}

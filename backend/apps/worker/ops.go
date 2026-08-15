package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5/pgxpool"
)

func runOpsLoop(ctx context.Context, db *pgxpool.Pool, client *asynq.Client) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	recoverStaleOperations(ctx, db, client)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			recoverStaleOperations(ctx, db, client)
			purgeOldInvocations(ctx, db)
			warnIfUnhealthy(ctx, db)
		}
	}
}

func recoverStaleOperations(ctx context.Context, db *pgxpool.Pool, client *asynq.Client) {
	if db == nil || client == nil {
		return
	}
	markExhaustedDecisions(ctx, db)
	recoverPrepareOperations(ctx, db, client)
	recoverNextTurnOperations(ctx, db, client)
	recoverReportOperations(ctx, db, client)
}

func markExhaustedDecisions(ctx context.Context, db *pgxpool.Pool) {
	_, err := db.Exec(ctx, `
		WITH exhausted AS (
			UPDATE interview_turn_decisions
			SET status='failed',error_code='DECISION_RECOVERY_EXHAUSTED',
			    error_summary='下一轮决策恢复次数已用尽，请手动重试',completed_at=now(),updated_at=now()
			WHERE status='running' AND attempt>=3 AND updated_at<now()-interval '10 minutes'
			RETURNING id,session_id,user_id
		), failed_tasks AS (
			UPDATE async_tasks task
			SET status='failed',error_code='DECISION_RECOVERY_EXHAUSTED',
			    error_summary='下一轮决策恢复次数已用尽，请手动重试',completed_at=now(),updated_at=now()
			FROM exhausted
			WHERE task.ref_id=exhausted.session_id AND task.user_id=exhausted.user_id
			  AND task.task_type='interview.next_turn' AND task.status='running'
		)
		UPDATE interview_sessions session
		SET phase='decision_failed',updated_at=now()
		FROM exhausted
		WHERE session.id=exhausted.session_id AND session.user_id=exhausted.user_id
		  AND session.status='active' AND session.phase='deciding'`)
	if err != nil {
		slog.Warn("decision recovery exhaustion update failed", "error", err.Error())
	}
}

func recoverPrepareOperations(ctx context.Context, db *pgxpool.Pool, client *asynq.Client) {
	rows, err := db.Query(ctx, `
		UPDATE async_tasks task SET status='pending',updated_at=now()
		FROM interview_sessions session JOIN question_sets qset ON qset.id=session.question_set_id
		WHERE task.ref_id=session.id AND task.user_id=session.user_id AND task.task_type='interview.prepare'
		  AND task.status IN ('pending','running')
		  AND (task.status='pending' OR task.updated_at<now()-interval '10 minutes')
		  AND task.created_at<now()-interval '30 seconds'
		  AND session.status='preparing' AND session.phase='preparing' AND qset.status='generating'
		RETURNING task.id::text,qset.id::text,session.id::text,session.user_id::text,
		          session.resume_id::text,session.resume_version_id::text,
		          session.primary_language,session.target_company,COALESCE(qset.target_role,'backend_development')`)
	if err != nil {
		slog.Warn("prepare recovery scan failed", "error", err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var payload sharedtasks.QuestionGeneratePayload
		if rows.Scan(&payload.TaskID, &payload.QuestionSetID, &payload.SessionID, &payload.UserID,
			&payload.ResumeID, &payload.ResumeVersionID, &payload.PrimaryLanguage, &payload.TargetCompany, &payload.TargetRole) != nil {
			continue
		}
		task, buildErr := sharedtasks.NewInterviewPrepareTask(payload)
		if buildErr == nil {
			_, buildErr = client.EnqueueContext(ctx, task, asynq.Queue("heavy"), asynq.Unique(10*time.Minute))
		}
		if buildErr != nil && !errors.Is(buildErr, asynq.ErrDuplicateTask) {
			slog.Warn("prepare recovery enqueue failed", "task_id", payload.TaskID, "error", buildErr.Error())
		}
	}
}

func recoverNextTurnOperations(ctx context.Context, db *pgxpool.Pool, client *asynq.Client) {
	rows, err := db.Query(ctx, `
		WITH recoverable AS (
			SELECT task.id,task.user_id,task.ref_id,decision.id AS decision_id,decision.answered_turn_id
			FROM async_tasks task
			JOIN interview_sessions session ON session.id=task.ref_id AND session.user_id=task.user_id
			JOIN interview_turns turn ON turn.session_id=session.id AND turn.ordinal=session.current_ordinal
			JOIN interview_turn_decisions decision ON decision.answered_turn_id=turn.id AND decision.session_id=session.id
			WHERE task.task_type='interview.next_turn' AND task.status IN ('pending','running')
			  AND (task.status='pending' OR task.updated_at<now()-interval '10 minutes')
			  AND task.created_at<now()-interval '30 seconds'
			  AND decision.status IN ('pending','running') AND decision.attempt<3
			  AND session.status='active' AND session.phase='deciding'
		)
		UPDATE async_tasks task SET status='pending',updated_at=now()
		FROM recoverable
		WHERE task.id=recoverable.id
		RETURNING task.id::text,recoverable.decision_id::text,task.ref_id::text,
		          recoverable.answered_turn_id::text,task.user_id::text,
		          COALESCE((task.result->>'use_fallback')::boolean,false)`)
	if err != nil {
		slog.Warn("next-turn recovery scan failed", "error", err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var payload sharedtasks.InterviewNextTurnPayload
		if rows.Scan(&payload.TaskID, &payload.DecisionID, &payload.SessionID, &payload.AnsweredTurnID, &payload.UserID, &payload.UseFallback) != nil {
			continue
		}
		_, _ = db.Exec(ctx, `UPDATE interview_turn_decisions SET status='pending',updated_at=now() WHERE id=$1 AND status='running'`, payload.DecisionID)
		task, buildErr := sharedtasks.NewInterviewNextTurnTask(payload)
		if buildErr == nil {
			_, buildErr = client.EnqueueContext(ctx, task, asynq.Queue("default"), asynq.Unique(10*time.Minute))
		}
		if buildErr != nil && !errors.Is(buildErr, asynq.ErrDuplicateTask) {
			slog.Warn("next-turn recovery enqueue failed", "task_id", payload.TaskID, "error", buildErr.Error())
		}
	}
}

func recoverReportOperations(ctx context.Context, db *pgxpool.Pool, client *asynq.Client) {
	rows, err := db.Query(ctx, `
		UPDATE async_tasks task SET status='pending',updated_at=now()
		FROM interview_reports report JOIN interview_sessions session ON session.id=report.session_id
		WHERE task.ref_id=session.id AND task.user_id=session.user_id AND task.task_type='report.generate'
		  AND task.status IN ('pending','running')
		  AND (task.status='pending' OR task.updated_at<now()-interval '10 minutes')
		  AND task.created_at<now()-interval '30 seconds'
		  AND session.status='completed' AND report.status IN ('pending','running')
		RETURNING task.id::text,session.id::text,session.user_id::text,report.id::text`)
	if err != nil {
		slog.Warn("report recovery scan failed", "error", err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var payload sharedtasks.ReportGeneratePayload
		if rows.Scan(&payload.TaskID, &payload.SessionID, &payload.UserID, &payload.ReportID) != nil {
			continue
		}
		task, buildErr := sharedtasks.NewReportGenerateTask(payload)
		if buildErr == nil {
			_, buildErr = client.EnqueueContext(ctx, task, asynq.Queue("heavy"), asynq.Unique(10*time.Minute))
		}
		if buildErr != nil && !errors.Is(buildErr, asynq.ErrDuplicateTask) {
			slog.Warn("report recovery enqueue failed", "task_id", payload.TaskID, "error", buildErr.Error())
		}
	}
}

func purgeOldInvocations(ctx context.Context, db *pgxpool.Pool) {
	tag, err := db.Exec(ctx, `DELETE FROM model_invocations WHERE created_at < now() - interval '90 days'`)
	if err != nil {
		slog.Warn("model invocation retention failed", "error", err.Error())
		return
	}
	if tag.RowsAffected() > 0 {
		slog.Info("purged expired model invocations", "deleted", tag.RowsAffected())
	}
}

func warnIfUnhealthy(ctx context.Context, db *pgxpool.Pool) {
	var failed5m, stalePending, staleRunning int
	err := db.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status::text = 'failed' AND updated_at > now() - interval '5 minutes'),
			count(*) FILTER (WHERE status::text IN ('pending') AND created_at < now() - interval '5 minutes'),
			count(*) FILTER (WHERE status::text = 'running' AND COALESCE(started_at, created_at) < now() - interval '10 minutes')
		FROM async_tasks`,
	).Scan(&failed5m, &stalePending, &staleRunning)
	if err != nil {
		slog.Warn("queue health query failed", "error", err.Error())
		return
	}
	if failed5m >= 8 {
		slog.Warn("async task failure spike", "failed_5m", failed5m)
	}
	if stalePending >= 10 {
		slog.Warn("async task queue backlog", "stale_pending", stalePending)
	}
	if staleRunning >= 5 {
		slog.Warn("async tasks stuck running", "stale_running", staleRunning)
	}

	var failedCalls, totalCalls int
	err = db.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'failed'),
			count(*)
		FROM model_invocations
		WHERE created_at > now() - interval '5 minutes'`,
	).Scan(&failedCalls, &totalCalls)
	if err != nil {
		return
	}
	if totalCalls >= 10 && failedCalls*100/totalCalls >= 20 {
		slog.Warn("model error rate above threshold", "failed", failedCalls, "total", totalCalls)
	}
}

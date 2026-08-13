package workspace

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/aiworkflow"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5"
)

func enqueueQuestionSetGeneration(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, resumeID, jobID, targetRole, sourceID string,
) (*types.TaskAcceptedResponse, error) {
	var versionID, resumeStatus string
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT COALESCE(current_version_id::text, ''), status::text
		FROM resumes WHERE id=$1 AND user_id=$2`, resumeID, userID,
	).Scan(&versionID, &resumeStatus)
	if err != nil {
		return nil, resourceNotFound("RESUME_NOT_FOUND", "未找到该简历", err)
	}
	if resumeStatus != "completed" || versionID == "" {
		return nil, conflict("RESUME_NOT_PARSED", "简历解析完成后才能生成题集", nil)
	}
	inputHash := aiworkflow.InputHash(versionID, jobID, strings.TrimSpace(targetRole), aiworkflow.PromptV1)
	var existingSetID, existingStatus string
	err = svcCtx.Database.QueryRow(ctx, `
		SELECT id::text, status::text
		FROM question_sets
		WHERE user_id=$1 AND input_hash=$2 AND status::text = ANY($3::text[])
		ORDER BY created_at DESC
		LIMIT 1`,
		userID, inputHash, []string{"generating", "ready", "degraded"},
	).Scan(&existingSetID, &existingStatus)
	if err == nil {
		return ensureQuestionTask(ctx, svcCtx, userID, existingSetID, resumeID, jobID, targetRole)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	setID := uuid.NewString()
	taskID := uuid.NewString()
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := validateQuestionSetReferences(ctx, tx, userID, resumeID, jobID); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO question_sets (
			id, user_id, resume_id, job_description_id, target_role, source_question_set_id, status, input_hash, prompt_version
		) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,'generating',$7,$8)`,
		setID, userID, resumeID, nullUUID(jobID), targetRole, nullUUID(sourceID), inputHash, aiworkflow.PromptV1,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO async_tasks (id, user_id, task_type, ref_id, status, progress)
		VALUES ($1,$2,'question.generate',$3,'pending',0)`,
		taskID, userID, setID,
	)
	if err != nil {
		return nil, err
	}
	queued, err := sharedtasks.NewQuestionGenerateTask(sharedtasks.QuestionGeneratePayload{
		TaskID:           taskID,
		QuestionSetID:    setID,
		UserID:           userID,
		ResumeID:         resumeID,
		JobDescriptionID: jobID,
		TargetRole:       targetRole,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if _, err := svcCtx.TaskClient.EnqueueContext(ctx, queued, asynq.Queue("heavy")); err != nil {
		_, _ = svcCtx.Database.Exec(ctx, `
			UPDATE async_tasks SET status='failed', error_code='TASK_ENQUEUE_FAILED', error_summary='任务暂时无法启动，请重试', error_message=$2, completed_at=now(), updated_at=now() WHERE id=$1`,
			taskID, err.Error())
		return nil, apperror.Unavailable("题集生成任务暂时无法启动，请重试", nil, err)
	}
	return &types.TaskAcceptedResponse{TaskId: taskID, Status: "pending"}, nil
}

func ensureQuestionTask(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, setID, resumeID, jobID, targetRole string,
) (*types.TaskAcceptedResponse, error) {
	var taskID, status string
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT id::text, status::text
		FROM async_tasks
		WHERE user_id=$1 AND ref_id=$2 AND task_type='question.generate'
		ORDER BY created_at DESC LIMIT 1`,
		userID, setID,
	).Scan(&taskID, &status)
	if err == nil {
		return &types.TaskAcceptedResponse{TaskId: taskID, Status: status}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	taskID = uuid.NewString()
	_, err = svcCtx.Database.Exec(ctx, `
		INSERT INTO async_tasks (id, user_id, task_type, ref_id, status, progress)
		VALUES ($1,$2,'question.generate',$3,'pending',0)`,
		taskID, userID, setID,
	)
	if isUniqueViolation(err) {
		err = svcCtx.Database.QueryRow(ctx, `
			SELECT id::text, status::text
			FROM async_tasks
			WHERE user_id=$1 AND ref_id=$2 AND task_type='question.generate'
			ORDER BY created_at DESC LIMIT 1`,
			userID, setID,
		).Scan(&taskID, &status)
		if err != nil {
			return nil, err
		}
		return &types.TaskAcceptedResponse{TaskId: taskID, Status: status}, nil
	}
	if err != nil {
		return nil, err
	}
	queued, err := sharedtasks.NewQuestionGenerateTask(sharedtasks.QuestionGeneratePayload{
		TaskID: taskID, QuestionSetID: setID, UserID: userID, ResumeID: resumeID, JobDescriptionID: jobID, TargetRole: targetRole,
	})
	if err != nil {
		return nil, err
	}
	if _, err := svcCtx.TaskClient.EnqueueContext(ctx, queued, asynq.Queue("heavy")); err != nil {
		return nil, apperror.Unavailable("题集生成任务暂时无法启动，请重试", nil, err)
	}
	return &types.TaskAcceptedResponse{TaskId: taskID, Status: "pending"}, nil
}

// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetryInterviewPreparationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// Retry a failed interview preparation without exposing an internal task
func NewRetryInterviewPreparationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetryInterviewPreparationLogic {
	return &RetryInterviewPreparationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetryInterviewPreparationLogic) RetryInterviewPreparation(req *types.InterviewPath) (resp *types.InterviewSessionResponse, err error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	tx, err := l.svcCtx.Database.Begin(l.ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(l.ctx)
	var questionSetID, resumeID, resumeVersionID, language, company, role, status, setStatus string
	err = tx.QueryRow(l.ctx, `
		SELECT qset.id::text,session.resume_id::text,session.resume_version_id::text,
		       session.primary_language,session.target_company,COALESCE(qset.target_role,'backend_development'),
		       session.status::text,qset.status::text
		FROM interview_sessions session JOIN question_sets qset ON qset.id=session.question_set_id
		WHERE session.id=$1 AND session.user_id=$2 FOR UPDATE OF session,qset`, req.Id, userID,
	).Scan(&questionSetID, &resumeID, &resumeVersionID, &language, &company, &role, &status, &setStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_NOT_FOUND", "未找到该面试", err)
	}
	if err != nil {
		return nil, err
	}
	if status != "failed" || setStatus != "failed" {
		return nil, conflict("INTERVIEW_PREPARATION_NOT_FAILED", "只有准备失败的面试可以重试", nil)
	}
	taskID := uuid.NewString()
	if _, err := tx.Exec(l.ctx, `
		UPDATE interview_sessions SET status='preparing',phase='preparing',updated_at=now() WHERE id=$1`, req.Id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(l.ctx, `UPDATE question_sets SET status='generating',updated_at=now() WHERE id=$1`, questionSetID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(l.ctx, `
		INSERT INTO async_tasks (id,user_id,task_type,ref_id,status,progress)
		VALUES ($1,$2,'interview.prepare',$3,'pending',0)`, taskID, userID, req.Id); err != nil {
		return nil, err
	}
	if err := tx.Commit(l.ctx); err != nil {
		return nil, err
	}
	payload := sharedtasks.QuestionGeneratePayload{
		TaskID: taskID, QuestionSetID: questionSetID, SessionID: req.Id, UserID: userID,
		ResumeID: resumeID, ResumeVersionID: resumeVersionID, PrimaryLanguage: language,
		TargetCompany: company, TargetRole: role,
	}
	queued, err := sharedtasks.NewInterviewPrepareTask(payload)
	if err == nil {
		_, err = l.svcCtx.TaskClient.EnqueueContext(l.ctx, queued, asynq.Queue("heavy"), asynq.Unique(10*time.Minute))
	}
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
		_, _ = l.svcCtx.Database.Exec(l.ctx, `
			UPDATE async_tasks SET status='failed',error_code='TASK_ENQUEUE_FAILED',error_summary='面试准备任务无法启动',
			    error_message=$2,completed_at=now(),updated_at=now() WHERE id=$1`, taskID, err.Error())
		_, _ = l.svcCtx.Database.Exec(l.ctx, `UPDATE question_sets SET status='failed',updated_at=now() WHERE id=$1`, questionSetID)
		_, _ = l.svcCtx.Database.Exec(l.ctx, `UPDATE interview_sessions SET status='failed',phase='preparing',updated_at=now() WHERE id=$1`, req.Id)
		return nil, apperror.Unavailable("面试准备任务暂时无法启动，请重试", nil, err)
	}
	return loadInterview(l.ctx, l.svcCtx, userID, req.Id)
}

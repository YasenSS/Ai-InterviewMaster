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

type RetryInterviewDecisionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// Retry the failed decision for the current answered turn
func NewRetryInterviewDecisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetryInterviewDecisionLogic {
	return &RetryInterviewDecisionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetryInterviewDecisionLogic) RetryInterviewDecision(req *types.InterviewPath) (resp *types.InterviewSessionResponse, err error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return restartInterviewDecision(l.ctx, l.svcCtx, userID, req.Id, false)
}

func restartInterviewDecision(ctx context.Context, svcCtx *svc.ServiceContext, userID, sessionID string, useFallback bool) (*types.InterviewSessionResponse, error) {
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var decisionID, turnID, decisionStatus string
	err = tx.QueryRow(ctx, `
		SELECT decision.id::text,turn.id::text,decision.status
		FROM interview_sessions session
		JOIN interview_turns turn ON turn.session_id=session.id AND turn.ordinal=session.current_ordinal
		JOIN interview_turn_decisions decision ON decision.answered_turn_id=turn.id
		WHERE session.id=$1 AND session.user_id=$2 AND session.status='active' AND session.phase='decision_failed'
		FOR UPDATE OF session,decision`, sessionID, userID,
	).Scan(&decisionID, &turnID, &decisionStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, conflict("INTERVIEW_DECISION_NOT_FAILED", "当前没有可重试的面试决策", nil)
	}
	if err != nil {
		return nil, err
	}
	if decisionStatus != "failed" {
		return nil, conflict("INTERVIEW_DECISION_NOT_FAILED", "当前决策不处于失败状态", nil)
	}
	taskID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		UPDATE interview_turn_decisions
		SET status='pending',error_code=NULL,error_summary=NULL,completed_at=NULL,updated_at=now()
		WHERE id=$1`, decisionID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE interview_sessions SET phase='deciding',updated_at=now() WHERE id=$1`, sessionID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO async_tasks (id,user_id,task_type,ref_id,status,progress,result)
		VALUES ($1,$2,'interview.next_turn',$3,'pending',0,jsonb_build_object('use_fallback',$4))`, taskID, userID, sessionID, useFallback); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	payload := sharedtasks.InterviewNextTurnPayload{
		TaskID: taskID, DecisionID: decisionID, SessionID: sessionID,
		AnsweredTurnID: turnID, UserID: userID, UseFallback: useFallback,
	}
	queued, err := sharedtasks.NewInterviewNextTurnTask(payload)
	if err == nil {
		_, err = svcCtx.TaskClient.EnqueueContext(ctx, queued, asynq.Queue("default"), asynq.Unique(10*time.Minute))
	}
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
		compensateNextTurnEnqueue(ctx, svcCtx, payload, err)
		return nil, apperror.Unavailable("下一轮问题暂时无法生成，请重试", nil, err)
	}
	return loadInterview(ctx, svcCtx, userID, sessionID)
}

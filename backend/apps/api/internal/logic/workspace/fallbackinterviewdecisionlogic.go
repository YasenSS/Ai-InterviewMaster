package workspace

import (
	"context"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type FallbackInterviewDecisionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFallbackInterviewDecisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FallbackInterviewDecisionLogic {
	return &FallbackInterviewDecisionLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *FallbackInterviewDecisionLogic) FallbackInterviewDecision(req *types.InterviewPath) (*types.InterviewSessionResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return restartInterviewDecision(l.ctx, l.svcCtx, userID, req.Id, true)
}

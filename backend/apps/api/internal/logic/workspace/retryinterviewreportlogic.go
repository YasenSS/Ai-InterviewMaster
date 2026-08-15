// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetryInterviewReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// Retry a failed report evaluation without exposing an internal task
func NewRetryInterviewReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetryInterviewReportLogic {
	return &RetryInterviewReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetryInterviewReportLogic) RetryInterviewReport(req *types.InterviewPath) (resp *types.InterviewReportResponse, err error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	reportID, err := retryReportGeneration(l.ctx, l.svcCtx, userID, req.Id)
	if err != nil {
		return nil, err
	}
	return loadInterviewReport(l.ctx, l.svcCtx, userID, req.Id, reportID)
}

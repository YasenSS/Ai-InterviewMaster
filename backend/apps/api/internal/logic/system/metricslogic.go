package system

import (
	"context"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/zeromicro/go-zero/core/logx"
)

type MetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MetricsLogic {
	return &MetricsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MetricsLogic) Metrics() (*types.MetricsResponse, error) {
	snap := platformai.MetricsSnapshot()
	return &types.MetricsResponse{
		Requests:                snap.Requests,
		Successes:               snap.Successes,
		Failures:                snap.Failures,
		Retries:                 snap.Retries,
		Degraded:                snap.Degraded,
		BudgetRejected:          snap.BudgetRejected,
		PromptTokens:            snap.PromptTokens,
		CompletionTokens:        snap.CompletionTokens,
		TotalTokens:             snap.TotalTokens,
		EstimatedCostMicros:     snap.EstimatedCostMicros,
		LatencyMsSum:            snap.LatencyMSSum,
		StructuredFirstFail:     snap.StructuredFirstFail,
		StructuredRepairSuccess: snap.StructuredRepairSuccess,
		StructuredFinalFail:     snap.StructuredFinalFail,
	}, nil
}

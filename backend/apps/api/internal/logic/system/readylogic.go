// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package system

import (
	"context"
	"fmt"
	"time"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReadyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// Dependency readiness probe
func NewReadyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReadyLogic {
	return &ReadyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReadyLogic) Ready() (resp *types.ReadinessResponse, err error) {
	checkCtx, cancel := context.WithTimeout(l.ctx, 2*time.Second)
	defer cancel()

	databaseStatus := "ok"
	redisStatus := "ok"
	var causes []error
	if err := l.svcCtx.Database.Ping(checkCtx); err != nil {
		databaseStatus = "unavailable"
		causes = append(causes, fmt.Errorf("postgresql: %w", err))
	}
	if err := l.svcCtx.Redis.Ping(checkCtx).Err(); err != nil {
		redisStatus = "unavailable"
		causes = append(causes, fmt.Errorf("redis: %w", err))
	}

	if len(causes) > 0 {
		return nil, apperror.Unavailable("The service dependencies are not ready.", map[string]string{
			"database": databaseStatus,
			"redis":    redisStatus,
		}, fmt.Errorf("readiness checks failed: %v", causes))
	}

	return &types.ReadinessResponse{
		Status:    "ok",
		Database:  databaseStatus,
		Redis:     redisStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

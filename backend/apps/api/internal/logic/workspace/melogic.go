// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"fmt"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MeLogic {
	return &MeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MeLogic) Me() (resp *types.UserResponse, err error) {
	userID, err := currentUserID(l.ctx)
	if err != nil { return nil, err }
	resp = &types.UserResponse{}
	err = l.svcCtx.Database.QueryRow(l.ctx, `SELECT id::text, email, display_name FROM users WHERE id = $1`, userID).Scan(&resp.Id, &resp.Email, &resp.DisplayName)
	if err != nil { return nil, fmt.Errorf("load current user: %w", err) }
	return resp, nil
}

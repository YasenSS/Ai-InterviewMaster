// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetInterviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetInterviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetInterviewLogic {
	return &GetInterviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetInterviewLogic) GetInterview(req *types.InterviewPath) (resp *types.InterviewSessionResponse, err error) {
	userID,err:=currentUserID(l.ctx);if err!=nil{return nil,err};return loadInterview(l.ctx,l.svcCtx.Database,userID,req.Id)
}

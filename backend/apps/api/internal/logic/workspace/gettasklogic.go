// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTaskLogic {
	return &GetTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetTaskLogic) GetTask(req *types.TaskPath) (resp *types.TaskResponse, err error) {
	userID,err:=currentUserID(l.ctx);if err!=nil{return nil,err};resp=&types.TaskResponse{};var raw []byte
	err=l.svcCtx.Database.QueryRow(l.ctx,`SELECT id::text,task_type,status::text,progress,COALESCE(error_message,''),COALESCE(result,'null'::jsonb) FROM async_tasks WHERE id=$1 AND user_id=$2`,req.Id,userID).Scan(&resp.Id,&resp.Type,&resp.Status,&resp.Progress,&resp.Error,&raw);if err!=nil{return nil,fmt.Errorf("task not found: %w",err)};_ = json.Unmarshal(raw,&resp.Result);return resp,nil
}

// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"fmt"
	"strings"
	"github.com/google/uuid"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/hibiken/asynq"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateASRTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateASRTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateASRTaskLogic {
	return &CreateASRTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateASRTaskLogic) CreateASRTask(req *types.ASRRequest) (resp *types.ASRResponse, err error) {
	userID,err:=currentUserID(l.ctx);if err!=nil{return nil,err};if strings.TrimSpace(req.ObjectKey)==""{return nil,fmt.Errorf("object_key is required")};taskID:=uuid.NewString();refID:=uuid.NewString();_,err=l.svcCtx.Database.Exec(l.ctx,`INSERT INTO async_tasks(id,user_id,task_type,ref_id,status,progress) VALUES($1,$2,'asr.transcribe',$3,'pending',0)`,taskID,userID,refID);if err!=nil{return nil,err};task,e:=sharedtasks.NewASRTask(sharedtasks.ASRPayload{TaskID:taskID,UserID:userID,ObjectKey:req.ObjectKey,Language:req.Language});if e!=nil{return nil,e};if _,e=l.svcCtx.TaskClient.EnqueueContext(l.ctx,task,asynq.Queue("heavy"));e!=nil{return nil,e};return &types.ASRResponse{TaskId:taskID,Status:"pending"},nil
}

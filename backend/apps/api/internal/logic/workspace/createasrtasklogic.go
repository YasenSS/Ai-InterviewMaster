package workspace

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/minio/minio-go/v7"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateASRTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateASRTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateASRTaskLogic {
	return &CreateASRTaskLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateASRTaskLogic) CreateASRTask(req *types.ASRRequest) (*types.ASRResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	objectKey := strings.TrimSpace(req.ObjectKey)
	if !strings.HasPrefix(objectKey, "users/"+userID+"/audio/") {
		return nil, resourceNotFound("ASR_UPLOAD_NOT_FOUND", "未找到该音频文件", nil)
	}
	if _, err := l.svcCtx.ObjectStore.StatObject(
		l.ctx,
		l.svcCtx.Config.Runtime.ObjectStore.Bucket,
		objectKey,
		minio.StatObjectOptions{},
	); err != nil {
		return nil, resourceNotFound("ASR_UPLOAD_NOT_FOUND", "未找到该音频文件", err)
	}
	taskID := uuid.NewString()
	refID := uuid.NewString()
	_, err = l.svcCtx.Database.Exec(l.ctx, `
		INSERT INTO async_tasks(id,user_id,task_type,ref_id,status,progress)
		VALUES($1,$2,'asr.transcribe',$3,'pending',0)`,
		taskID,
		userID,
		refID,
	)
	if err != nil {
		return nil, err
	}
	task, err := sharedtasks.NewASRTask(sharedtasks.ASRPayload{
		TaskID:    taskID,
		UserID:    userID,
		ObjectKey: objectKey,
		Language:  req.Language,
	})
	if err == nil {
		_, err = l.svcCtx.TaskClient.EnqueueContext(l.ctx, task, asynq.Queue("heavy"))
	}
	if err != nil {
		_, _ = l.svcCtx.Database.Exec(l.ctx, `
			UPDATE async_tasks
			SET status='failed',
			    error_code='TASK_ENQUEUE_FAILED',
			    error_summary='任务暂时无法启动，请重试',
			    error_message=$2,
			    completed_at=now(),
			    updated_at=now()
			WHERE id=$1`,
			taskID,
			err.Error(),
		)
		return nil, err
	}
	return &types.ASRResponse{TaskId: taskID, Status: "pending"}, nil
}

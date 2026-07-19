// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"fmt"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	workertasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/hibiken/asynq"
	"github.com/minio/minio-go/v7"

	"github.com/zeromicro/go-zero/core/logx"
)

type CompleteResumeUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCompleteResumeUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteResumeUploadLogic {
	return &CompleteResumeUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CompleteResumeUploadLogic) CompleteResumeUpload(req *types.CompleteResumeUploadRequest) (resp *types.CompleteResumeUploadResponse, err error) {
	userID, err := currentUserID(l.ctx); if err != nil { return nil, err }
	var objectKey string
	err = l.svcCtx.Database.QueryRow(l.ctx, `SELECT rv.object_key FROM resumes r JOIN resume_versions rv ON rv.id=$2 WHERE r.id=$1 AND r.user_id=$3`, req.Id,req.VersionId,userID).Scan(&objectKey); if err != nil { return nil, fmt.Errorf("resume version not found: %w",err) }
	if _, err := l.svcCtx.ObjectStore.StatObject(l.ctx,l.svcCtx.Config.Runtime.ObjectStore.Bucket,objectKey,minio.StatObjectOptions{}); err != nil { return nil, fmt.Errorf("uploaded file not found: %w",err) }
	var taskID string
	err = l.svcCtx.Database.QueryRow(l.ctx, `INSERT INTO async_tasks (user_id,task_type,ref_id) VALUES ($1,'resume.parse',$2) RETURNING id::text`,userID,req.VersionId).Scan(&taskID); if err != nil { return nil, err }
	task, err := workertasks.NewResumeParseTask(workertasks.ResumeParsePayload{TaskID:taskID,ResumeID:req.Id,VersionID:req.VersionId,UserID:userID}); if err != nil { return nil,err }
	if _,err=l.svcCtx.TaskClient.EnqueueContext(l.ctx,task,asynq.Queue("heavy")); err != nil{return nil,err}
	_,err=l.svcCtx.Database.Exec(l.ctx,`UPDATE resumes SET status='pending',updated_at=now() WHERE id=$1`,req.Id); if err != nil{return nil,err}
	return &types.CompleteResumeUploadResponse{TaskId:taskID,Status:"pending"},nil
}

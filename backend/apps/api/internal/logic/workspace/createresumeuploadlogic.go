// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/objectstore"
	"github.com/google/uuid"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateResumeUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateResumeUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateResumeUploadLogic {
	return &CreateResumeUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateResumeUploadLogic) CreateResumeUpload(req *types.CreateResumeUploadRequest) (resp *types.CreateResumeUploadResponse, err error) {
	userID, err := currentUserID(l.ctx); if err != nil { return nil, err }
	if strings.TrimSpace(req.FileName) == "" || req.SizeBytes <= 0 || req.SizeBytes > 20*1024*1024 { return nil, fmt.Errorf("请选择不超过 20MB 的简历文件") }
	ext := strings.ToLower(filepath.Ext(req.FileName)); if ext != ".pdf" && ext != ".docx" && ext != ".txt" { return nil, fmt.Errorf("仅支持 PDF、DOCX 或 TXT 简历") }
	if err := objectstore.EnsureBucket(l.ctx, l.svcCtx.ObjectStore, l.svcCtx.Config.Runtime.ObjectStore.Bucket, l.svcCtx.Config.Runtime.ObjectStore.Region); err != nil { return nil, err }
	resumeID, versionID := uuid.NewString(), uuid.NewString()
	objectKey := fmt.Sprintf("users/%s/resumes/%s/%s%s", userID, resumeID, versionID, ext)
	tx, err := l.svcCtx.Database.Begin(l.ctx); if err != nil { return nil, err }; defer tx.Rollback(l.ctx)
	_, err = tx.Exec(l.ctx, `INSERT INTO resumes (id,user_id,title,status) VALUES ($1,$2,$3,'uploading')`, resumeID, userID, strings.TrimSpace(req.Title)); if err != nil { return nil, err }
	_, err = tx.Exec(l.ctx, `INSERT INTO resume_versions (id,resume_id,version_no,object_key,original_filename,content_type,size_bytes) VALUES ($1,$2,1,$3,$4,$5,$6)`, versionID,resumeID,objectKey,req.FileName,req.ContentType,req.SizeBytes); if err != nil { return nil, err }
	_, err = tx.Exec(l.ctx, `UPDATE resumes SET current_version_id=$2 WHERE id=$1`, resumeID, versionID); if err != nil { return nil, err }
	if err := tx.Commit(l.ctx); err != nil { return nil, err }
	putURL, err := l.svcCtx.ObjectStore.PresignedPutObject(l.ctx, l.svcCtx.Config.Runtime.ObjectStore.Bucket, objectKey, 15*time.Minute); if err != nil { return nil, fmt.Errorf("sign upload: %w", err) }
	return &types.CreateResumeUploadResponse{ResumeId:resumeID,VersionId:versionID,UploadUrl:putURL.String(),UploadHeaders:map[string]string{"Content-Type":req.ContentType},ExpiresAt:time.Now().Add(15*time.Minute).UTC().Format(time.RFC3339)}, nil
}

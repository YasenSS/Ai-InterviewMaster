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

type ListResumesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListResumesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListResumesLogic {
	return &ListResumesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListResumesLogic) ListResumes() (resp []types.ResumeResponse, err error) {
	userID, err := currentUserID(l.ctx); if err != nil{return nil,err}
	rows,err:=l.svcCtx.Database.Query(l.ctx,`SELECT r.id::text,r.title,r.status::text,COALESCE(rv.id::text,''),COALESCE(rv.original_filename,''),r.created_at::text,r.updated_at::text FROM resumes r LEFT JOIN resume_versions rv ON rv.id=r.current_version_id WHERE r.user_id=$1 ORDER BY r.updated_at DESC`,userID); if err!=nil{return nil,fmt.Errorf("list resumes: %w",err)}; defer rows.Close()
	for rows.Next(){var item types.ResumeResponse; if err:=rows.Scan(&item.Id,&item.Title,&item.Status,&item.VersionId,&item.FileName,&item.CreatedAt,&item.UpdatedAt);err!=nil{return nil,err};resp=append(resp,item)}
	return resp,rows.Err()
}

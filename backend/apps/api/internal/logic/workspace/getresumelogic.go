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

type GetResumeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetResumeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResumeLogic {
	return &GetResumeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetResumeLogic) GetResume(req *types.ResumePath) (resp *types.ResumeDetailResponse, err error) {
	userID,err:=currentUserID(l.ctx);if err!=nil{return nil,err}; resp=&types.ResumeDetailResponse{}
	err=l.svcCtx.Database.QueryRow(l.ctx,`SELECT r.id::text,r.title,r.status::text,COALESCE(rv.id::text,''),COALESCE(rv.original_filename,''),r.created_at::text,r.updated_at::text FROM resumes r LEFT JOIN resume_versions rv ON rv.id=r.current_version_id WHERE r.id=$1 AND r.user_id=$2`,req.Id,userID).Scan(&resp.Id,&resp.Title,&resp.Status,&resp.VersionId,&resp.FileName,&resp.CreatedAt,&resp.UpdatedAt);if err!=nil{return nil,fmt.Errorf("resume not found: %w",err)}
	if resp.VersionId=="" {return resp,nil}; rows,err:=l.svcCtx.Database.Query(l.ctx,`SELECT fact_type,fact_key,fact_value,source_excerpt FROM resume_facts WHERE resume_version_id=$1 ORDER BY created_at`,resp.VersionId);if err!=nil{return nil,err};defer rows.Close()
	for rows.Next(){var fact types.ResumeFactResponse;var raw []byte;if err:=rows.Scan(&fact.Type,&fact.Key,&raw,&fact.Excerpt);err!=nil{return nil,err};if err:=json.Unmarshal(raw,&fact.Value);err!=nil{return nil,err};resp.Facts=append(resp.Facts,fact)};return resp,rows.Err()
}

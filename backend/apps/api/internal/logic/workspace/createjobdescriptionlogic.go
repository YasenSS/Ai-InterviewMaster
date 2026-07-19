// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"fmt"
	"strings"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateJobDescriptionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateJobDescriptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateJobDescriptionLogic {
	return &CreateJobDescriptionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateJobDescriptionLogic) CreateJobDescription(req *types.JobDescriptionRequest) (resp *types.JobDescriptionResponse, err error) {
	userID, err := currentUserID(l.ctx)
	if err != nil { return nil, err }
	if strings.TrimSpace(req.Title) == "" || len(strings.TrimSpace(req.Content)) < 20 { return nil, fmt.Errorf("岗位名称和至少 20 字的 JD 内容为必填项") }
	capabilities := extractCapabilities(req.Content)
	encoded, err := capabilitiesJSON(capabilities)
	if err != nil { return nil, err }
	resp = &types.JobDescriptionResponse{Capabilities: capabilities}
	err = l.svcCtx.Database.QueryRow(l.ctx, `
		INSERT INTO job_descriptions (user_id, company, title, content, extracted_capabilities)
		VALUES ($1, NULLIF($2, ''), $3, $4, $5) RETURNING id::text, COALESCE(company, ''), title, content, created_at::text, updated_at::text`,
		userID, strings.TrimSpace(req.Company), strings.TrimSpace(req.Title), strings.TrimSpace(req.Content), encoded).
		Scan(&resp.Id, &resp.Company, &resp.Title, &resp.Content, &resp.CreatedAt, &resp.UpdatedAt)
	if err != nil { return nil, fmt.Errorf("create job description: %w", err) }
	return resp, nil
}

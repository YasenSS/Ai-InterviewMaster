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

type ListJobDescriptionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListJobDescriptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListJobDescriptionsLogic {
	return &ListJobDescriptionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListJobDescriptionsLogic) ListJobDescriptions() (resp []types.JobDescriptionResponse, err error) {
	userID, err := currentUserID(l.ctx)
	if err != nil { return nil, err }
	rows, err := l.svcCtx.Database.Query(l.ctx, `SELECT id::text, COALESCE(company, ''), title, content, extracted_capabilities, created_at::text, updated_at::text FROM job_descriptions WHERE user_id=$1 ORDER BY updated_at DESC`, userID)
	if err != nil { return nil, fmt.Errorf("list job descriptions: %w", err) }
	defer rows.Close()
	for rows.Next() {
		var item types.JobDescriptionResponse; var raw []byte
		if err := rows.Scan(&item.Id, &item.Company, &item.Title, &item.Content, &raw, &item.CreatedAt, &item.UpdatedAt); err != nil { return nil, err }
		if err := json.Unmarshal(raw, &item.Capabilities); err != nil { return nil, err }
		resp = append(resp, item)
	}
	return resp, rows.Err()
}

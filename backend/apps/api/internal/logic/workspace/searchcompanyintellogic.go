// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchCompanyIntelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchCompanyIntelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchCompanyIntelLogic {
	return &SearchCompanyIntelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchCompanyIntelLogic) SearchCompanyIntel(req *types.CompanyIntelRequest) (resp *types.CompanyIntelResponse, err error) {
	if _, err := currentUserID(l.ctx); err != nil {
		return nil, err
	}
	company := strings.TrimSpace(req.Company)
	role := strings.TrimSpace(req.Role)
	if company == "" || role == "" {
		return nil, fmt.Errorf("company and role are required")
	}
	return &types.CompanyIntelResponse{Company: company, Role: role, Topics: []string{"项目深挖", "系统设计", "行为面试"}, Questions: []string{fmt.Sprintf("为什么选择 %s 的 %s 岗位？", company, role), "请设计一个可扩展的核心业务系统。", "描述一次线上故障的定位与复盘。"}, Freshness: time.Now().UTC().Format(time.RFC3339)}, nil
}

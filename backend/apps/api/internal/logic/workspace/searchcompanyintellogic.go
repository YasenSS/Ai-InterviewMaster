package workspace

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	aitools "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/tools"

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

func (l *SearchCompanyIntelLogic) SearchCompanyIntel(req *types.CompanyIntelRequest) (*types.CompanyIntelResponse, error) {
	if _, err := currentUserID(l.ctx); err != nil {
		return nil, err
	}
	company := strings.TrimSpace(req.Company)
	role := strings.TrimSpace(req.Role)
	if company == "" || role == "" {
		return nil, fmt.Errorf("company and role are required")
	}
	hits, err := aitools.SearchIntel(l.ctx, l.svcCtx.Database, company, role, "", 8)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		hits, err = aitools.SearchIntel(l.ctx, l.svcCtx.Database, "", role, "", 8)
		if err != nil {
			return nil, err
		}
	}
	topics := make([]string, 0, len(hits))
	questions := make([]string, 0, len(hits))
	seen := map[string]struct{}{}
	for _, hit := range hits {
		if _, ok := seen[hit.Topic]; !ok {
			seen[hit.Topic] = struct{}{}
			topics = append(topics, hit.Topic)
		}
		questions = append(questions, hit.Summary)
	}
	if len(topics) == 0 {
		questions = []string{"本地语料暂无该公司条目，未使用网络抓取。可改用更通用的岗位关键词重试。"}
	}
	return &types.CompanyIntelResponse{
		Company:   company,
		Role:      role,
		Topics:    topics,
		Questions: questions,
		Freshness: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

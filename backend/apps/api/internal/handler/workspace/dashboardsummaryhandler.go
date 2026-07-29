// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"net/http"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/logic/workspace"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func DashboardSummaryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := workspace.NewDashboardSummaryLogic(r.Context(), svcCtx)
		resp, err := l.DashboardSummary()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

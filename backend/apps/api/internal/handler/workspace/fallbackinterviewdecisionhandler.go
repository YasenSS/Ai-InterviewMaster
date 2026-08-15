// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"net/http"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/logic/workspace"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// Continue a failed decision with an explicitly marked fallback question
func FallbackInterviewDecisionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.InterviewPath
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := workspace.NewFallbackInterviewDecisionLogic(r.Context(), svcCtx)
		resp, err := l.FallbackInterviewDecision(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusAccepted, resp)
		}
	}
}

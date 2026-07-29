package auth

import (
	"net/http"

	authlogic "github.com/interviewmaster/interviewmaster/backend/apps/api/internal/logic/auth"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func RefreshHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logic := authlogic.NewRefreshLogic(r.Context(), svcCtx)
		response, newRefreshToken, err := logic.Refresh(readRefreshCookie(r, svcCtx.Config))
		if err != nil {
			http.SetCookie(w, clearRefreshCookie(svcCtx.Config))
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		http.SetCookie(w, refreshCookie(svcCtx.Config, newRefreshToken))
		httpx.OkJsonCtx(r.Context(), w, response)
	}
}

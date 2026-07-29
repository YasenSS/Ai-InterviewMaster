package auth

import (
	"net/http"

	authlogic "github.com/interviewmaster/interviewmaster/backend/apps/api/internal/logic/auth"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func LogoutHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logic := authlogic.NewLogoutLogic(r.Context(), svcCtx)
		err := logic.Logout(readRefreshCookie(r, svcCtx.Config))
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		http.SetCookie(w, clearRefreshCookie(svcCtx.Config))
		httpx.Ok(w)
	}
}

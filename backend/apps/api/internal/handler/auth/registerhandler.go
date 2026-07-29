package auth

import (
	"net/http"

	authlogic "github.com/interviewmaster/interviewmaster/backend/apps/api/internal/logic/auth"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func RegisterHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RegisterRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		logic := authlogic.NewRegisterLogic(r.Context(), svcCtx)
		result, err := logic.RegisterWithSession(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		http.SetCookie(w, refreshCookie(svcCtx.Config, result.RefreshToken))
		httpx.OkJsonCtx(r.Context(), w, result.Response)
	}
}

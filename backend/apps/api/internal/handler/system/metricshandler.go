package system

import (
	"net/http"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/logic/system"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func MetricsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := system.NewMetricsLogic(r.Context(), svcCtx)
		resp, err := l.Metrics()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

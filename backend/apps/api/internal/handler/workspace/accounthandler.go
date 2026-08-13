package workspace

import (
	"net/http"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/logic/workspace"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ExportMeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := workspace.NewExportMeLogic(r.Context(), svcCtx)
		resp, err := l.ExportMe()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func DeleteMeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DeleteAccountRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := workspace.NewDeleteMeLogic(r.Context(), svcCtx)
		if err := l.DeleteMe(&req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.Ok(w)
	}
}

func GetSkillProfileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := workspace.NewGetSkillProfileLogic(r.Context(), svcCtx)
		resp, err := l.GetSkillProfile()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func UpdateSkillProfileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdateSkillProfileRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := workspace.NewUpdateSkillProfileLogic(r.Context(), svcCtx)
		resp, err := l.UpdateSkillProfile(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func DeleteSkillProfileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := workspace.NewDeleteSkillProfileLogic(r.Context(), svcCtx)
		if err := l.DeleteSkillProfile(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.Ok(w)
	}
}

// Resource logic is consolidated here so all resume endpoints share one domain implementation.
package workspace

import (
	"context"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateResumeUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateResumeUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateResumeUploadLogic {
	return &CreateResumeUploadLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateResumeUploadLogic) CreateResumeUpload(req *types.CreateResumeUploadRequest) (*types.CreateResumeUploadResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	return createResumeUpload(l.ctx, l.svcCtx, userID, req)
}

type CompleteResumeUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCompleteResumeUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteResumeUploadLogic {
	return &CompleteResumeUploadLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CompleteResumeUploadLogic) CompleteResumeUpload(req *types.CompleteResumeUploadRequest) (*types.ResumeDetailResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	if err := validateID("version_id", req.VersionId); err != nil {
		return nil, err
	}
	return completeResumeUpload(l.ctx, l.svcCtx, userID, req.Id, req.VersionId)
}

type ListResumesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListResumesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListResumesLogic {
	return &ListResumesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListResumesLogic) ListResumes(req *types.ResumeListRequest) (*types.ResumePageResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	return listResumes(l.ctx, l.svcCtx, userID, req.Status, req.Page, req.PageSize, req.Sort)
}

type GetResumeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetResumeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResumeLogic {
	return &GetResumeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetResumeLogic) GetResume(req *types.ResumePath) (*types.ResumeDetailResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return loadResume(l.ctx, l.svcCtx, userID, req.Id)
}

type UpdateResumeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateResumeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateResumeLogic {
	return &UpdateResumeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateResumeLogic) UpdateResume(req *types.UpdateResumeRequest) (*types.ResumeSummaryResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return updateResumeTitle(l.ctx, l.svcCtx, userID, req.Id, req.Title)
}

type DeleteResumeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteResumeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteResumeLogic {
	return &DeleteResumeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeleteResumeLogic) DeleteResume(req *types.ResumePath) error {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return err
	}
	if err := validateID("id", req.Id); err != nil {
		return err
	}
	return deleteResume(l.ctx, l.svcCtx, userID, req.Id)
}

type ReparseResumeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReparseResumeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReparseResumeLogic {
	return &ReparseResumeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ReparseResumeLogic) ReparseResume(req *types.ResumePath) (*types.ResumeDetailResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return reparseResume(l.ctx, l.svcCtx, userID, req.Id)
}

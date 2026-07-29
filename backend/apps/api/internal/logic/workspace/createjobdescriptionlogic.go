// Resource logic is consolidated here so all job endpoints share one domain implementation.
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/zeromicro/go-zero/core/logx"
)

var jobSorts = map[string]string{
	"updated_at_desc": "updated_at DESC, id DESC",
	"updated_at_asc":  "updated_at ASC, id ASC",
}

func validateJobInput(company, title, content string) (string, string, string, error) {
	company, err := validateOptionalText("company", company, 120)
	if err != nil {
		return "", "", "", err
	}
	title, err = validateTitle("title", title)
	if err != nil {
		return "", "", "", err
	}
	content = strings.TrimSpace(content)
	length := utf8.RuneCountInString(content)
	if length < 20 || length > 50000 {
		return "", "", "", apperror.Validation(map[string][]string{
			"content": {"JD 正文长度必须为 20–50,000 个字符"},
		})
	}
	return company, title, content, nil
}

func scanJob(row pgx.Row) (types.JobDescriptionResponse, error) {
	var item types.JobDescriptionResponse
	var raw []byte
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&item.Id,
		&item.Company,
		&item.Title,
		&item.Content,
		&raw,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return item, err
	}
	item.Capabilities = []string{}
	if err := json.Unmarshal(raw, &item.Capabilities); err != nil {
		return item, err
	}
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func createJob(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.CreateJobDescriptionRequest,
) (*types.JobDescriptionResponse, error) {
	company, title, content, err := validateJobInput(req.Company, req.Title, req.Content)
	if err != nil {
		return nil, err
	}
	capabilities := extractCapabilities(content)
	raw, err := capabilitiesJSON(capabilities)
	if err != nil {
		return nil, err
	}
	item, err := scanJob(svcCtx.Database.QueryRow(ctx, `
		INSERT INTO job_descriptions (
			user_id, company, title, content, extracted_capabilities
		)
		VALUES ($1, NULLIF($2, ''), $3, $4, $5)
		RETURNING id::text,
		          COALESCE(company, ''),
		          title,
		          content,
		          extracted_capabilities,
		          created_at,
		          updated_at`,
		userID,
		company,
		title,
		content,
		raw,
	))
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func listJobs(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.PageRequest,
) (*types.JobDescriptionPageResponse, error) {
	page, pageSize, offset, err := pageParams(req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	orderBy, err := sortClause(req.Sort, "updated_at_desc", jobSorts)
	if err != nil {
		return nil, err
	}
	response := &types.JobDescriptionPageResponse{
		Items:    []types.JobDescriptionResponse{},
		Page:     page,
		PageSize: pageSize,
	}
	if err := svcCtx.Database.QueryRow(
		ctx,
		`SELECT count(*) FROM job_descriptions WHERE user_id = $1`,
		userID,
	).Scan(&response.Total); err != nil {
		return nil, err
	}
	rows, err := svcCtx.Database.Query(ctx, `
		SELECT id::text,
		       COALESCE(company, ''),
		       title,
		       content,
		       extracted_capabilities,
		       created_at,
		       updated_at
		FROM job_descriptions
		WHERE user_id = $1
		ORDER BY `+orderBy+`
		LIMIT $2 OFFSET $3`,
		userID,
		pageSize,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		response.Items = append(response.Items, item)
	}
	return response, rows.Err()
}

func getJob(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, jobID string,
) (*types.JobDescriptionResponse, error) {
	item, err := scanJob(svcCtx.Database.QueryRow(ctx, `
		SELECT id::text,
		       COALESCE(company, ''),
		       title,
		       content,
		       extracted_capabilities,
		       created_at,
		       updated_at
		FROM job_descriptions
		WHERE id = $1 AND user_id = $2`,
		jobID,
		userID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("JOB_DESCRIPTION_NOT_FOUND", "未找到该职位描述", err)
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func updateJob(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.UpdateJobDescriptionRequest,
) (*types.JobDescriptionResponse, error) {
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var company, title, content string
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(company, ''), title, content
		FROM job_descriptions
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`,
		req.Id,
		userID,
	).Scan(&company, &title, &content)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("JOB_DESCRIPTION_NOT_FOUND", "未找到该职位描述", err)
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Company) == "" &&
		strings.TrimSpace(req.Title) == "" &&
		strings.TrimSpace(req.Content) == "" {
		return nil, apperror.Validation(map[string][]string{
			"body": {"至少提供一个需要更新的字段"},
		})
	}
	if req.Company != "" {
		company = req.Company
	}
	if req.Title != "" {
		title = req.Title
	}
	contentChanged := false
	if req.Content != "" {
		content = req.Content
		contentChanged = true
	}
	company, title, content, err = validateJobInput(company, title, content)
	if err != nil {
		return nil, err
	}
	var raw []byte
	if contentChanged || req.Title != "" {
		raw, err = capabilitiesJSON(extractCapabilities(content))
		if err != nil {
			return nil, err
		}
	} else {
		err = tx.QueryRow(ctx, `
			SELECT extracted_capabilities
			FROM job_descriptions
			WHERE id = $1`,
			req.Id,
		).Scan(&raw)
		if err != nil {
			return nil, err
		}
	}
	var item types.JobDescriptionResponse
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
		UPDATE job_descriptions
		SET company = NULLIF($3, ''),
		    title = $4,
		    content = $5,
		    extracted_capabilities = $6,
		    updated_at = now()
		WHERE id = $1 AND user_id = $2
		RETURNING id::text,
		          COALESCE(company, ''),
		          title,
		          content,
		          created_at,
		          updated_at`,
		req.Id,
		userID,
		company,
		title,
		content,
		raw,
	).Scan(
		&item.Id,
		&item.Company,
		&item.Title,
		&item.Content,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	item.Capabilities = decodeStrings(raw)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &item, nil
}

func deleteJob(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, jobID string,
) error {
	_, err := svcCtx.Database.Exec(ctx, `
		DELETE FROM job_descriptions
		WHERE id = $1 AND user_id = $2`,
		jobID,
		userID,
	)
	return err
}

type CreateJobDescriptionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateJobDescriptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateJobDescriptionLogic {
	return &CreateJobDescriptionLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateJobDescriptionLogic) CreateJobDescription(req *types.CreateJobDescriptionRequest) (*types.JobDescriptionResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	return createJob(l.ctx, l.svcCtx, userID, req)
}

type ListJobDescriptionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListJobDescriptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListJobDescriptionsLogic {
	return &ListJobDescriptionsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListJobDescriptionsLogic) ListJobDescriptions(req *types.PageRequest) (*types.JobDescriptionPageResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	return listJobs(l.ctx, l.svcCtx, userID, req)
}

type GetJobDescriptionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetJobDescriptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetJobDescriptionLogic {
	return &GetJobDescriptionLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetJobDescriptionLogic) GetJobDescription(req *types.JobDescriptionPath) (*types.JobDescriptionResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return getJob(l.ctx, l.svcCtx, userID, req.Id)
}

type UpdateJobDescriptionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateJobDescriptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateJobDescriptionLogic {
	return &UpdateJobDescriptionLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateJobDescriptionLogic) UpdateJobDescription(req *types.UpdateJobDescriptionRequest) (*types.JobDescriptionResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return updateJob(l.ctx, l.svcCtx, userID, req)
}

type DeleteJobDescriptionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteJobDescriptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteJobDescriptionLogic {
	return &DeleteJobDescriptionLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeleteJobDescriptionLogic) DeleteJobDescription(req *types.JobDescriptionPath) error {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return err
	}
	if err := validateID("id", req.Id); err != nil {
		return err
	}
	return deleteJob(l.ctx, l.svcCtx, userID, req.Id)
}

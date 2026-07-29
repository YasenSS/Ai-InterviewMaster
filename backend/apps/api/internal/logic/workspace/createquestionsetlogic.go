// Resource logic is consolidated here so all question-set endpoints share one domain implementation.
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/zeromicro/go-zero/core/logx"
)

var questionSetStatuses = map[string]struct{}{"ready": {}, "archived": {}}

var questionSetSorts = map[string]string{
	"created_at_desc": "qs.created_at DESC, qs.id DESC",
	"created_at_asc":  "qs.created_at ASC, qs.id ASC",
}

func generatedQuestions() []types.QuestionInput {
	return []types.QuestionInput{
		{
			Ordinal:        1,
			Question:       "请用两分钟介绍你自己，并说明与目标岗位最匹配的经历。",
			Intent:         "自我认知与岗位匹配",
			ExpectedPoints: []string{"核心经历", "量化结果", "岗位关联"},
			FollowUpHint:   "其中哪项成果最能代表你的个人贡献？",
		},
		{
			Ordinal:        2,
			Question:       "选择简历中最有挑战的项目，说明背景、目标、行动与结果。",
			Intent:         "项目深挖",
			ExpectedPoints: []string{"STAR 结构", "技术决策", "结果指标"},
			FollowUpHint:   "如果重新做一次，你会改变什么？",
		},
		{
			Ordinal:        3,
			Question:       "项目中遇到过什么技术故障或性能瓶颈？你如何定位？",
			Intent:         "问题解决",
			ExpectedPoints: []string{"定位路径", "证据数据", "复盘改进"},
			FollowUpHint:   "如何证明修复真正有效？",
		},
		{
			Ordinal:        4,
			Question:       "描述一次你与团队成员意见不一致的经历。",
			Intent:         "协作沟通",
			ExpectedPoints: []string{"分歧背景", "沟通方式", "共同结果"},
			FollowUpHint:   "你如何处理仍未达成一致的部分？",
		},
		{
			Ordinal:        5,
			Question:       "为什么申请这个岗位？未来一年希望获得什么成长？",
			Intent:         "动机与规划",
			ExpectedPoints: []string{"岗位理解", "能力差距", "行动计划"},
			FollowUpHint:   "你入职前三个月会优先做什么？",
		},
	}
}

func validateQuestionInputs(inputs []types.QuestionInput) ([]types.QuestionInput, error) {
	if len(inputs) < 1 || len(inputs) > 50 {
		return nil, apperror.Validation(map[string][]string{
			"questions": {"题目数量必须为 1–50"},
		})
	}
	values := append([]types.QuestionInput(nil), inputs...)
	sort.Slice(values, func(i, j int) bool { return values[i].Ordinal < values[j].Ordinal })
	for index := range values {
		item := &values[index]
		if item.Ordinal != index+1 {
			return nil, apperror.Validation(map[string][]string{
				"questions": {"题号必须从 1 开始且唯一、连续"},
			})
		}
		item.Question = strings.TrimSpace(item.Question)
		item.Intent = strings.TrimSpace(item.Intent)
		item.FollowUpHint = strings.TrimSpace(item.FollowUpHint)
		if length := utf8.RuneCountInString(item.Question); length < 1 || length > 2000 {
			return nil, apperror.Validation(map[string][]string{
				"questions": {"题目正文长度必须为 1–2,000 个字符"},
			})
		}
		if length := utf8.RuneCountInString(item.Intent); length < 1 || length > 1000 {
			return nil, apperror.Validation(map[string][]string{
				"questions": {"考察意图长度必须为 1–1,000 个字符"},
			})
		}
		if utf8.RuneCountInString(item.FollowUpHint) > 1000 {
			return nil, apperror.Validation(map[string][]string{
				"questions": {"追问提示不能超过 1,000 个字符"},
			})
		}
		if len(item.ExpectedPoints) > 20 {
			return nil, apperror.Validation(map[string][]string{
				"questions": {"每道题最多包含 20 个期望回答点"},
			})
		}
		for pointIndex := range item.ExpectedPoints {
			item.ExpectedPoints[pointIndex] = strings.TrimSpace(item.ExpectedPoints[pointIndex])
			length := utf8.RuneCountInString(item.ExpectedPoints[pointIndex])
			if length < 1 || length > 500 {
				return nil, apperror.Validation(map[string][]string{
					"questions": {"期望回答点长度必须为 1–500 个字符"},
				})
			}
		}
		if item.ExpectedPoints == nil {
			item.ExpectedPoints = []string{}
		}
	}
	return values, nil
}

func validateQuestionSetReferences(
	ctx context.Context,
	tx pgx.Tx,
	userID, resumeID, jobID string,
) error {
	var resumeStatus string
	err := tx.QueryRow(ctx, `
		SELECT status::text
		FROM resumes
		WHERE id = $1 AND user_id = $2`,
		resumeID,
		userID,
	).Scan(&resumeStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return resourceNotFound("RESUME_NOT_FOUND", "未找到该简历", err)
	}
	if err != nil {
		return err
	}
	if resumeStatus != "completed" {
		return conflict("RESUME_NOT_PARSED", "简历解析完成后才能生成题集", nil)
	}
	if strings.TrimSpace(jobID) != "" {
		var exists bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM job_descriptions
				WHERE id = $1 AND user_id = $2
			)`,
			jobID,
			userID,
		).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return resourceNotFound("JOB_DESCRIPTION_NOT_FOUND", "未找到该职位描述", nil)
		}
	}
	return nil
}

func insertQuestionSet(
	ctx context.Context,
	tx pgx.Tx,
	userID, resumeID, jobID, targetRole, sourceID string,
	questions []types.QuestionInput,
) (string, error) {
	setID := uuid.NewString()
	_, err := tx.Exec(ctx, `
		INSERT INTO question_sets (
			id, user_id, resume_id, job_description_id, target_role, source_question_set_id
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)`,
		setID,
		userID,
		resumeID,
		nullUUID(jobID),
		strings.TrimSpace(targetRole),
		nullUUID(sourceID),
	)
	if err != nil {
		return "", err
	}
	for _, item := range questions {
		_, err = tx.Exec(ctx, `
			INSERT INTO questions (
				id, question_set_id, ordinal, question, intent, expected_points, follow_up_hint
			)
			VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))`,
			uuid.NewString(),
			setID,
			item.Ordinal,
			item.Question,
			item.Intent,
			encodeStrings(item.ExpectedPoints),
			item.FollowUpHint,
		)
		if err != nil {
			return "", err
		}
	}
	return setID, nil
}

func createQuestionSet(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.CreateQuestionSetRequest,
) (*types.QuestionSetDetailResponse, error) {
	if err := validateID("resume_id", req.ResumeId); err != nil {
		return nil, err
	}
	if req.JobDescriptionId != "" {
		if err := validateID("job_description_id", req.JobDescriptionId); err != nil {
			return nil, err
		}
	}
	targetRole, err := validateOptionalText("target_role", req.TargetRole, 120)
	if err != nil {
		return nil, err
	}
	questions, err := validateQuestionInputs(generatedQuestions())
	if err != nil {
		return nil, err
	}
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := validateQuestionSetReferences(
		ctx,
		tx,
		userID,
		req.ResumeId,
		req.JobDescriptionId,
	); err != nil {
		return nil, err
	}
	setID, err := insertQuestionSet(
		ctx,
		tx,
		userID,
		req.ResumeId,
		req.JobDescriptionId,
		targetRole,
		"",
		questions,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return loadQuestionSet(ctx, svcCtx, userID, setID)
}

type questionSetSummaryScanner interface {
	Scan(dest ...any) error
}

func scanQuestionSetSummary(row questionSetSummaryScanner) (types.QuestionSetSummaryResponse, error) {
	var item types.QuestionSetSummaryResponse
	var jobID, jobCompany, jobTitle string
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&item.Id,
		&item.Resume.Id,
		&item.Resume.Title,
		&jobID,
		&jobCompany,
		&jobTitle,
		&item.TargetRole,
		&item.Status,
		&item.QuestionCount,
		&item.SourceQuestionSetId,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return item, err
	}
	if jobID != "" {
		item.JobDescription = &types.JobDescriptionReferenceResponse{
			Id:      jobID,
			Company: jobCompany,
			Title:   jobTitle,
		}
	}
	item.ResumeId = item.Resume.Id
	item.JobDescriptionId = jobID
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

const questionSetSummarySelect = `
	SELECT qs.id::text,
	       resume.id::text,
	       resume.title,
	       COALESCE(job.id::text, ''),
	       COALESCE(job.company, ''),
	       COALESCE(job.title, ''),
	       COALESCE(qs.target_role, ''),
	       qs.status::text,
	       count(question.id)::int,
	       COALESCE(qs.source_question_set_id::text, ''),
	       qs.created_at,
	       qs.updated_at
	FROM question_sets AS qs
	JOIN resumes AS resume ON resume.id = qs.resume_id
	LEFT JOIN job_descriptions AS job ON job.id = qs.job_description_id
	LEFT JOIN questions AS question ON question.question_set_id = qs.id
`

func loadQuestionSet(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, setID string,
) (*types.QuestionSetDetailResponse, error) {
	summary, err := scanQuestionSetSummary(svcCtx.Database.QueryRow(ctx,
		questionSetSummarySelect+`
		WHERE qs.id = $1 AND qs.user_id = $2
		GROUP BY qs.id, resume.id, job.id`,
		setID,
		userID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("QUESTION_SET_NOT_FOUND", "未找到该题集", err)
	}
	if err != nil {
		return nil, err
	}
	response := &types.QuestionSetDetailResponse{
		QuestionSetSummaryResponse: summary,
		Questions:                  []types.QuestionResponse{},
	}
	rows, err := svcCtx.Database.Query(ctx, `
		SELECT id::text,
		       ordinal,
		       question,
		       intent,
		       expected_points,
		       COALESCE(follow_up_hint, '')
		FROM questions
		WHERE question_set_id = $1
		ORDER BY ordinal ASC`,
		setID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item types.QuestionResponse
		var raw []byte
		if err := rows.Scan(
			&item.Id,
			&item.Ordinal,
			&item.Question,
			&item.Intent,
			&raw,
			&item.FollowUpHint,
		); err != nil {
			return nil, err
		}
		item.ExpectedPoints = []string{}
		if err := json.Unmarshal(raw, &item.ExpectedPoints); err != nil {
			return nil, err
		}
		response.Questions = append(response.Questions, item)
	}
	return response, rows.Err()
}

func listQuestionSets(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.QuestionSetListRequest,
) (*types.QuestionSetPageResponse, error) {
	page, pageSize, offset, err := pageParams(req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	statuses, err := parseEnumFilter("status", req.Status, questionSetStatuses)
	if err != nil {
		return nil, err
	}
	if statuses == nil {
		statuses = []string{}
	}
	if req.ResumeId != "" {
		if err := validateID("resume_id", req.ResumeId); err != nil {
			return nil, err
		}
	}
	orderBy, err := sortClause(req.Sort, "created_at_desc", questionSetSorts)
	if err != nil {
		return nil, err
	}
	response := &types.QuestionSetPageResponse{
		Items:    []types.QuestionSetSummaryResponse{},
		Page:     page,
		PageSize: pageSize,
	}
	err = svcCtx.Database.QueryRow(ctx, `
		SELECT count(*)
		FROM question_sets
		WHERE user_id = $1
		  AND (cardinality($2::text[]) = 0 OR status::text = ANY($2::text[]))
		  AND ($3 = '' OR resume_id::text = $3)`,
		userID,
		statuses,
		req.ResumeId,
	).Scan(&response.Total)
	if err != nil {
		return nil, err
	}
	rows, err := svcCtx.Database.Query(ctx,
		questionSetSummarySelect+`
		WHERE qs.user_id = $1
		  AND (cardinality($2::text[]) = 0 OR qs.status::text = ANY($2::text[]))
		  AND ($3 = '' OR qs.resume_id::text = $3)
		GROUP BY qs.id, resume.id, job.id
		ORDER BY `+orderBy+`
		LIMIT $4 OFFSET $5`,
		userID,
		statuses,
		req.ResumeId,
		pageSize,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanQuestionSetSummary(rows)
		if err != nil {
			return nil, err
		}
		response.Items = append(response.Items, item)
	}
	return response, rows.Err()
}

func updateQuestionSet(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.UpdateQuestionSetRequest,
) (*types.QuestionSetDetailResponse, error) {
	if req.TargetRole == "" && req.Status == "" && req.Questions == nil {
		return nil, apperror.Validation(map[string][]string{
			"body": {"至少提供一个需要更新的字段"},
		})
	}
	var questions []types.QuestionInput
	var err error
	if req.Questions != nil {
		questions, err = validateQuestionInputs(req.Questions)
		if err != nil {
			return nil, err
		}
	}
	targetRole := strings.TrimSpace(req.TargetRole)
	if targetRole != "" {
		targetRole, err = validateOptionalText("target_role", targetRole, 120)
		if err != nil {
			return nil, err
		}
	}
	if req.Status != "" {
		if _, ok := questionSetStatuses[req.Status]; !ok {
			return nil, apperror.Validation(map[string][]string{
				"status": {"必须是 ready 或 archived"},
			})
		}
	}
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var existingTarget, existingStatus string
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(target_role, ''), status::text
		FROM question_sets
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`,
		req.Id,
		userID,
	).Scan(&existingTarget, &existingStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("QUESTION_SET_NOT_FOUND", "未找到该题集", err)
	}
	if err != nil {
		return nil, err
	}
	if targetRole == "" {
		targetRole = existingTarget
	}
	status := req.Status
	if status == "" {
		status = existingStatus
	}
	_, err = tx.Exec(ctx, `
		UPDATE question_sets
		SET target_role = NULLIF($3, ''),
		    status = $4::question_set_status,
		    updated_at = now()
		WHERE id = $1 AND user_id = $2`,
		req.Id,
		userID,
		targetRole,
		status,
	)
	if err != nil {
		return nil, err
	}
	if req.Questions != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM questions WHERE question_set_id = $1`, req.Id); err != nil {
			return nil, err
		}
		for _, item := range questions {
			_, err = tx.Exec(ctx, `
				INSERT INTO questions (
					id, question_set_id, ordinal, question, intent, expected_points, follow_up_hint
				)
				VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))`,
				uuid.NewString(),
				req.Id,
				item.Ordinal,
				item.Question,
				item.Intent,
				encodeStrings(item.ExpectedPoints),
				item.FollowUpHint,
			)
			if err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return loadQuestionSet(ctx, svcCtx, userID, req.Id)
}

func deleteQuestionSet(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, setID string,
) error {
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var lockedID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM question_sets
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`,
		setID,
		userID,
	).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var interviewCount int
	if err := tx.QueryRow(
		ctx,
		`SELECT count(*) FROM interview_sessions WHERE question_set_id = $1`,
		setID,
	).Scan(&interviewCount); err != nil {
		return err
	}
	if interviewCount > 0 {
		return conflict(
			"QUESTION_SET_IN_USE",
			"该题集已有历史面试，请归档而不是删除",
			map[string]any{"interview_count": interviewCount},
		)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM question_sets WHERE id = $1`, setID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func regenerateQuestionSet(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.RegenerateQuestionSetRequest,
) (*types.QuestionSetDetailResponse, error) {
	var resumeID, jobID, targetRole string
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT resume_id::text,
		       COALESCE(job_description_id::text, ''),
		       COALESCE(target_role, '')
		FROM question_sets
		WHERE id = $1 AND user_id = $2`,
		req.Id,
		userID,
	).Scan(&resumeID, &jobID, &targetRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("QUESTION_SET_NOT_FOUND", "未找到该题集", err)
	}
	if err != nil {
		return nil, err
	}
	if req.JobDescriptionId != "" {
		jobID = req.JobDescriptionId
	}
	if req.TargetRole != "" {
		targetRole = req.TargetRole
	}
	targetRole, err = validateOptionalText("target_role", targetRole, 120)
	if err != nil {
		return nil, err
	}
	questions, err := validateQuestionInputs(generatedQuestions())
	if err != nil {
		return nil, err
	}
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := validateQuestionSetReferences(ctx, tx, userID, resumeID, jobID); err != nil {
		return nil, err
	}
	newID, err := insertQuestionSet(
		ctx,
		tx,
		userID,
		resumeID,
		jobID,
		targetRole,
		req.Id,
		questions,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return loadQuestionSet(ctx, svcCtx, userID, newID)
}

type CreateQuestionSetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateQuestionSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateQuestionSetLogic {
	return &CreateQuestionSetLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateQuestionSetLogic) CreateQuestionSet(req *types.CreateQuestionSetRequest) (*types.QuestionSetDetailResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	return createQuestionSet(l.ctx, l.svcCtx, userID, req)
}

type ListQuestionSetsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListQuestionSetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListQuestionSetsLogic {
	return &ListQuestionSetsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListQuestionSetsLogic) ListQuestionSets(req *types.QuestionSetListRequest) (*types.QuestionSetPageResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	return listQuestionSets(l.ctx, l.svcCtx, userID, req)
}

type GetQuestionSetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetQuestionSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetQuestionSetLogic {
	return &GetQuestionSetLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetQuestionSetLogic) GetQuestionSet(req *types.QuestionSetPath) (*types.QuestionSetDetailResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return loadQuestionSet(l.ctx, l.svcCtx, userID, req.Id)
}

type UpdateQuestionSetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateQuestionSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateQuestionSetLogic {
	return &UpdateQuestionSetLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateQuestionSetLogic) UpdateQuestionSet(req *types.UpdateQuestionSetRequest) (*types.QuestionSetDetailResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return updateQuestionSet(l.ctx, l.svcCtx, userID, req)
}

type DeleteQuestionSetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteQuestionSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteQuestionSetLogic {
	return &DeleteQuestionSetLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeleteQuestionSetLogic) DeleteQuestionSet(req *types.QuestionSetPath) error {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return err
	}
	if err := validateID("id", req.Id); err != nil {
		return err
	}
	return deleteQuestionSet(l.ctx, l.svcCtx, userID, req.Id)
}

type RegenerateQuestionSetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegenerateQuestionSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegenerateQuestionSetLogic {
	return &RegenerateQuestionSetLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RegenerateQuestionSetLogic) RegenerateQuestionSet(req *types.RegenerateQuestionSetRequest) (*types.QuestionSetDetailResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	if req.JobDescriptionId != "" {
		if err := validateID("job_description_id", req.JobDescriptionId); err != nil {
			return nil, err
		}
	}
	return regenerateQuestionSet(l.ctx, l.svcCtx, userID, req)
}

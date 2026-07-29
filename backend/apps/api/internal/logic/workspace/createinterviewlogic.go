// Resource logic is consolidated here so all interview state changes share one transaction model.
package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/zeromicro/go-zero/core/logx"
)

var interviewStatuses = map[string]struct{}{
	"draft": {}, "active": {}, "completed": {}, "abandoned": {},
}

var interviewSorts = map[string]string{
	"updated_at_desc": "session.updated_at DESC, session.id DESC",
	"updated_at_asc":  "session.updated_at ASC, session.id ASC",
	"created_at_desc": "session.created_at DESC, session.id DESC",
}

type interviewSummaryScanner interface {
	Scan(dest ...any) error
}

const interviewSummarySelect = `
	SELECT session.id::text,
	       session.title,
	       session.status::text,
	       COALESCE(qset.id::text, ''),
	       COALESCE(qset.target_role, ''),
	       resume.id::text,
	       resume.title,
	       count(turn.id)::int,
	       count(turn.id) FILTER (WHERE turn.answer IS NOT NULL)::int,
	       count(turn.id) FILTER (WHERE turn.answer IS NULL AND turn.skipped_at IS NOT NULL)::int,
	       session.current_ordinal,
	       report.overall_score::int,
	       session.started_at,
	       session.completed_at,
	       CASE
	           WHEN session.started_at IS NULL THEN session.duration_seconds
	           WHEN session.status = 'active' THEN GREATEST(
	               0,
	               FLOOR(EXTRACT(EPOCH FROM (now() - session.started_at)))::integer
	           )
	           WHEN session.completed_at IS NOT NULL THEN GREATEST(
	               session.duration_seconds,
	               FLOOR(EXTRACT(EPOCH FROM (session.completed_at - session.started_at)))::integer
	           )
	           ELSE session.duration_seconds
	       END,
	       session.created_at,
	       session.updated_at
	FROM interview_sessions AS session
	JOIN resumes AS resume ON resume.id = session.resume_id
	LEFT JOIN question_sets AS qset ON qset.id = session.question_set_id
	LEFT JOIN interview_turns AS turn ON turn.session_id = session.id
	LEFT JOIN interview_reports AS report ON report.session_id = session.id
`

func scanInterviewSummary(row interviewSummaryScanner) (types.InterviewSummaryResponse, error) {
	var item types.InterviewSummaryResponse
	var questionSetID, targetRole string
	var startedAt, completedAt *time.Time
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&item.Id,
		&item.Title,
		&item.Status,
		&questionSetID,
		&targetRole,
		&item.Resume.Id,
		&item.Resume.Title,
		&item.QuestionCount,
		&item.AnsweredCount,
		&item.SkippedCount,
		&item.CurrentOrdinal,
		&item.OverallScore,
		&startedAt,
		&completedAt,
		&item.DurationSeconds,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return item, err
	}
	if questionSetID != "" {
		item.QuestionSet = &types.QuestionSetReferenceResponse{Id: questionSetID, TargetRole: targetRole}
	}
	item.StartedAt = formatOptionalTime(startedAt)
	item.CompletedAt = formatOptionalTime(completedAt)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func loadInterview(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
) (*types.InterviewSessionResponse, error) {
	summary, err := scanInterviewSummary(svcCtx.Database.QueryRow(ctx,
		interviewSummarySelect+`
		WHERE session.id = $1 AND session.user_id = $2
		GROUP BY session.id, resume.id, qset.id, report.id`,
		sessionID,
		userID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_NOT_FOUND", "未找到该面试", err)
	}
	if err != nil {
		return nil, err
	}
	response := &types.InterviewSessionResponse{
		InterviewSummaryResponse: summary,
		Turns:                    []types.InterviewTurnResponse{},
	}
	var jobID, company, jobTitle string
	err = svcCtx.Database.QueryRow(ctx, `
		SELECT COALESCE(job.id::text, ''),
		       COALESCE(job.company, ''),
		       COALESCE(job.title, ''),
		       session.question_duration_seconds
		FROM interview_sessions AS session
		LEFT JOIN job_descriptions AS job ON job.id = session.job_description_id
		WHERE session.id = $1 AND session.user_id = $2`,
		sessionID,
		userID,
	).Scan(&jobID, &company, &jobTitle, &response.QuestionDurationSeconds)
	if err != nil {
		return nil, err
	}
	if jobID != "" {
		response.JobDescription = &types.JobDescriptionReferenceResponse{
			Id:      jobID,
			Company: company,
			Title:   jobTitle,
		}
	}
	rows, err := svcCtx.Database.Query(ctx, `
		SELECT ordinal,
		       question,
		       COALESCE(answer, ''),
		       CASE
		           WHEN answer IS NOT NULL THEN 'answered'
		           WHEN skipped_at IS NOT NULL THEN 'skipped'
		           WHEN started_at IS NOT NULL THEN 'answering'
		           ELSE 'unstarted'
		       END,
		       started_at,
		       answered_at,
		       skipped_at,
		       CASE
		           WHEN $2 = 'active'
		                AND started_at IS NOT NULL
		                AND answer IS NULL
		                AND skipped_at IS NULL
		           THEN GREATEST(
		               time_spent_seconds,
		               FLOOR(EXTRACT(EPOCH FROM (now() - started_at)))::integer
		           )
		           ELSE time_spent_seconds
		       END
		FROM interview_turns
		WHERE session_id = $1
		ORDER BY ordinal ASC`,
		sessionID,
		summary.Status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item types.InterviewTurnResponse
		var startedAt, answeredAt, skippedAt *time.Time
		if err := rows.Scan(
			&item.Ordinal,
			&item.Question,
			&item.Answer,
			&item.State,
			&startedAt,
			&answeredAt,
			&skippedAt,
			&item.TimeSpentSeconds,
		); err != nil {
			return nil, err
		}
		item.StartedAt = formatOptionalTime(startedAt)
		item.AnsweredAt = formatOptionalTime(answeredAt)
		item.SkippedAt = formatOptionalTime(skippedAt)
		response.Turns = append(response.Turns, item)
	}
	return response, rows.Err()
}

func createInterview(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.CreateInterviewRequest,
) (*types.InterviewSessionResponse, error) {
	if err := validateID("resume_id", req.ResumeId); err != nil {
		return nil, err
	}
	if err := validateID("question_set_id", req.QuestionSetId); err != nil {
		return nil, err
	}
	if req.JobDescriptionId != "" {
		if err := validateID("job_description_id", req.JobDescriptionId); err != nil {
			return nil, err
		}
	}
	title, err := validateTitle("title", req.Title)
	if err != nil {
		return nil, err
	}
	duration := req.QuestionDurationSeconds
	if duration == 0 {
		duration = 180
	}
	if duration != 180 {
		return nil, apperror.Validation(map[string][]string{
			"question_duration_seconds": {"当前仅支持 180 秒"},
		})
	}
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var questionCount int
	err = tx.QueryRow(ctx, `
		SELECT count(question.id)::int
		FROM question_sets AS qset
		LEFT JOIN questions AS question ON question.question_set_id = qset.id
		WHERE qset.id = $1
		  AND qset.user_id = $2
		  AND qset.resume_id = $3
		GROUP BY qset.id`,
		req.QuestionSetId,
		userID,
		req.ResumeId,
	).Scan(&questionCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("QUESTION_SET_NOT_FOUND", "未找到与该简历关联的题集", err)
	}
	if err != nil {
		return nil, err
	}
	if questionCount == 0 {
		return nil, conflict("QUESTION_SET_EMPTY", "题集没有可用于面试的题目", nil)
	}
	if req.JobDescriptionId != "" {
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM job_descriptions WHERE id = $1 AND user_id = $2
			)`,
			req.JobDescriptionId,
			userID,
		).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, resourceNotFound("JOB_DESCRIPTION_NOT_FOUND", "未找到该职位描述", nil)
		}
	}
	sessionID := uuid.NewString()
	_, err = tx.Exec(ctx, `
		INSERT INTO interview_sessions (
			id,
			user_id,
			resume_id,
			question_set_id,
			job_description_id,
			title,
			status,
			current_ordinal,
			started_at,
			question_duration_seconds
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'active', 1, now(), $7)`,
		sessionID,
		userID,
		req.ResumeId,
		req.QuestionSetId,
		nullUUID(req.JobDescriptionId),
		title,
		duration,
	)
	if err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO interview_turns (
			session_id, ordinal, question, started_at
		)
		SELECT $1,
		       ordinal,
		       question,
		       CASE WHEN ordinal = 1 THEN now() ELSE NULL END
		FROM questions
		WHERE question_set_id = $2
		ORDER BY ordinal ASC`,
		sessionID,
		req.QuestionSetId,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, conflict("QUESTION_SET_EMPTY", "题集没有可用于面试的题目", nil)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return loadInterview(ctx, svcCtx, userID, sessionID)
}

func listInterviews(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.InterviewListRequest,
) (*types.InterviewPageResponse, error) {
	page, pageSize, offset, err := pageParams(req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	statuses, err := parseEnumFilter("status", req.Status, interviewStatuses)
	if err != nil {
		return nil, err
	}
	if statuses == nil {
		statuses = []string{}
	}
	orderBy, err := sortClause(req.Sort, "updated_at_desc", interviewSorts)
	if err != nil {
		return nil, err
	}
	response := &types.InterviewPageResponse{
		Items:    []types.InterviewSummaryResponse{},
		Page:     page,
		PageSize: pageSize,
	}
	err = svcCtx.Database.QueryRow(ctx, `
		SELECT count(*)
		FROM interview_sessions
		WHERE user_id = $1
		  AND (cardinality($2::text[]) = 0 OR status::text = ANY($2::text[]))`,
		userID,
		statuses,
	).Scan(&response.Total)
	if err != nil {
		return nil, err
	}
	rows, err := svcCtx.Database.Query(ctx,
		interviewSummarySelect+`
		WHERE session.user_id = $1
		  AND (cardinality($2::text[]) = 0 OR session.status::text = ANY($2::text[]))
		GROUP BY session.id, resume.id, qset.id, report.id
		ORDER BY `+orderBy+`
		LIMIT $3 OFFSET $4`,
		userID,
		statuses,
		pageSize,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanInterviewSummary(rows)
		if err != nil {
			return nil, err
		}
		response.Items = append(response.Items, item)
	}
	return response, rows.Err()
}

func lockActiveInterview(
	ctx context.Context,
	tx pgx.Tx,
	userID, sessionID string,
) (string, int, error) {
	var status string
	var currentOrdinal int
	err := tx.QueryRow(ctx, `
		SELECT status::text, current_ordinal
		FROM interview_sessions
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`,
		sessionID,
		userID,
	).Scan(&status, &currentOrdinal)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, resourceNotFound("INTERVIEW_NOT_FOUND", "未找到该面试", err)
	}
	if err != nil {
		return "", 0, err
	}
	if status == "completed" {
		return "", 0, conflict("INTERVIEW_ALREADY_COMPLETED", "已完成的面试不能再修改", nil)
	}
	if status != "active" {
		return "", 0, conflict("INTERVIEW_NOT_ACTIVE", "当前面试状态不允许修改", nil)
	}
	return status, currentOrdinal, nil
}

func advanceInterview(
	ctx context.Context,
	tx pgx.Tx,
	sessionID string,
) error {
	var nextOrdinal int
	err := tx.QueryRow(ctx, `
		SELECT ordinal
		FROM interview_turns
		WHERE session_id = $1
		  AND answer IS NULL
		  AND skipped_at IS NULL
		ORDER BY ordinal ASC
		LIMIT 1`,
		sessionID,
	).Scan(&nextOrdinal)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.QueryRow(ctx, `
			UPDATE interview_sessions
			SET current_ordinal = (
			        SELECT COALESCE(max(ordinal), 0)
			        FROM interview_turns
			        WHERE session_id = $1
			    ),
			    updated_at = now()
			WHERE id = $1
			RETURNING current_ordinal`,
			sessionID,
		).Scan(&nextOrdinal)
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE interview_turns
		SET started_at = COALESCE(started_at, now())
		WHERE session_id = $1 AND ordinal = $2`,
		sessionID,
		nextOrdinal,
	); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE interview_sessions
		SET current_ordinal = $2, updated_at = now()
		WHERE id = $1`,
		sessionID,
		nextOrdinal,
	)
	return err
}

func saveInterviewAnswer(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
	ordinal int,
	answer string,
) (*types.InterviewSessionResponse, error) {
	answer, err := validateAnswer(answer)
	if err != nil {
		return nil, err
	}
	if ordinal < 1 {
		return nil, apperror.Validation(map[string][]string{
			"ordinal": {"必须大于等于 1"},
		})
	}
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, _, err := lockActiveInterview(ctx, tx, userID, sessionID); err != nil {
		return nil, err
	}
	var turnID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM interview_turns
		WHERE session_id = $1 AND ordinal = $2
		FOR UPDATE`,
		sessionID,
		ordinal,
	).Scan(&turnID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_TURN_NOT_FOUND", "未找到该面试题", err)
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE interview_turns
		SET started_at = COALESCE(started_at, now()),
		    answer = $3,
		    answered_at = now(),
		    skipped_at = NULL,
		    time_spent_seconds = GREATEST(
		        time_spent_seconds,
		        FLOOR(EXTRACT(EPOCH FROM (now() - COALESCE(started_at, now()))))::integer
		    )
		WHERE session_id = $1 AND ordinal = $2`,
		sessionID,
		ordinal,
		answer,
	)
	if err != nil {
		return nil, err
	}
	if err := advanceInterview(ctx, tx, sessionID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return loadInterview(ctx, svcCtx, userID, sessionID)
}

func skipInterviewTurn(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
	ordinal int,
) (*types.InterviewSessionResponse, error) {
	if ordinal < 1 {
		return nil, apperror.Validation(map[string][]string{
			"ordinal": {"必须大于等于 1"},
		})
	}
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, _, err := lockActiveInterview(ctx, tx, userID, sessionID); err != nil {
		return nil, err
	}
	var answer string
	var skippedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(answer, ''), skipped_at
		FROM interview_turns
		WHERE session_id = $1 AND ordinal = $2
		FOR UPDATE`,
		sessionID,
		ordinal,
	).Scan(&answer, &skippedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_TURN_NOT_FOUND", "未找到该面试题", err)
	}
	if err != nil {
		return nil, err
	}
	if answer != "" {
		return nil, conflict("TURN_ALREADY_ANSWERED", "该题已有答案，不能直接跳过", nil)
	}
	if skippedAt == nil {
		_, err = tx.Exec(ctx, `
			UPDATE interview_turns
			SET started_at = COALESCE(started_at, now()),
			    skipped_at = now(),
			    time_spent_seconds = GREATEST(
			        time_spent_seconds,
			        FLOOR(EXTRACT(EPOCH FROM (now() - COALESCE(started_at, now()))))::integer
			    )
			WHERE session_id = $1 AND ordinal = $2`,
			sessionID,
			ordinal,
		)
		if err != nil {
			return nil, err
		}
	}
	if err := advanceInterview(ctx, tx, sessionID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return loadInterview(ctx, svcCtx, userID, sessionID)
}

func completeInterview(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
	confirmIncomplete bool,
) (*types.InterviewSessionResponse, error) {
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var status string
	err = tx.QueryRow(ctx, `
		SELECT status::text
		FROM interview_sessions
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`,
		sessionID,
		userID,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_NOT_FOUND", "未找到该面试", err)
	}
	if err != nil {
		return nil, err
	}
	if status == "completed" {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return loadInterview(ctx, svcCtx, userID, sessionID)
	}
	if status != "active" {
		return nil, conflict("INTERVIEW_NOT_ACTIVE", "当前面试状态不能完成", nil)
	}
	rows, err := tx.Query(ctx, `
		SELECT ordinal
		FROM interview_turns
		WHERE session_id = $1 AND answer IS NULL
		ORDER BY ordinal ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	unanswered := []int{}
	for rows.Next() {
		var ordinal int
		if err := rows.Scan(&ordinal); err != nil {
			rows.Close()
			return nil, err
		}
		unanswered = append(unanswered, ordinal)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(unanswered) > 0 && !confirmIncomplete {
		return nil, conflict(
			"INTERVIEW_HAS_UNANSWERED_TURNS",
			"仍有未回答的问题，请确认后再完成面试",
			map[string]any{"ordinals": unanswered},
		)
	}
	_, err = tx.Exec(ctx, `
		UPDATE interview_turns
		SET time_spent_seconds = GREATEST(
		        time_spent_seconds,
		        FLOOR(EXTRACT(EPOCH FROM (now() - started_at)))::integer
		    )
		WHERE session_id = $1
		  AND started_at IS NOT NULL
		  AND answer IS NULL
		  AND skipped_at IS NULL`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE interview_sessions
		SET status = 'completed',
		    completed_at = now(),
		    duration_seconds = GREATEST(
		        0,
		        FLOOR(EXTRACT(EPOCH FROM (now() - started_at)))::integer
		    ),
		    updated_at = now()
		WHERE id = $1`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return loadInterview(ctx, svcCtx, userID, sessionID)
}

type CreateInterviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateInterviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateInterviewLogic {
	return &CreateInterviewLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateInterviewLogic) CreateInterview(req *types.CreateInterviewRequest) (*types.InterviewSessionResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	return createInterview(l.ctx, l.svcCtx, userID, req)
}

type ListInterviewsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListInterviewsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInterviewsLogic {
	return &ListInterviewsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListInterviewsLogic) ListInterviews(req *types.InterviewListRequest) (*types.InterviewPageResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	return listInterviews(l.ctx, l.svcCtx, userID, req)
}

type GetInterviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetInterviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetInterviewLogic {
	return &GetInterviewLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetInterviewLogic) GetInterview(req *types.InterviewPath) (*types.InterviewSessionResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return loadInterview(l.ctx, l.svcCtx, userID, req.Id)
}

type SaveInterviewAnswerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveInterviewAnswerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveInterviewAnswerLogic {
	return &SaveInterviewAnswerLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *SaveInterviewAnswerLogic) SaveInterviewAnswer(req *types.SaveInterviewAnswerRequest) (*types.InterviewSessionResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return saveInterviewAnswer(l.ctx, l.svcCtx, userID, req.Id, req.Ordinal, req.Answer)
}

type SkipInterviewTurnLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSkipInterviewTurnLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SkipInterviewTurnLogic {
	return &SkipInterviewTurnLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *SkipInterviewTurnLogic) SkipInterviewTurn(req *types.InterviewTurnPath) (*types.InterviewSessionResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return skipInterviewTurn(l.ctx, l.svcCtx, userID, req.Id, req.Ordinal)
}

type CompleteInterviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCompleteInterviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteInterviewLogic {
	return &CompleteInterviewLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CompleteInterviewLogic) CompleteInterview(req *types.CompleteInterviewRequest) (*types.InterviewSessionResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	return completeInterview(l.ctx, l.svcCtx, userID, req.Id, req.ConfirmIncomplete)
}

type AnswerInterviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnswerInterviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnswerInterviewLogic {
	return &AnswerInterviewLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AnswerInterviewLogic) AnswerInterview(req *types.AnswerInterviewRequest) (*types.InterviewSessionResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	var ordinal int
	err = l.svcCtx.Database.QueryRow(l.ctx, `
		SELECT current_ordinal
		FROM interview_sessions
		WHERE id = $1 AND user_id = $2`,
		req.Id,
		userID,
	).Scan(&ordinal)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_NOT_FOUND", "未找到该面试", err)
	}
	if err != nil {
		return nil, err
	}
	return saveInterviewAnswer(l.ctx, l.svcCtx, userID, req.Id, ordinal, req.Answer)
}

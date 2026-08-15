// Resource logic is consolidated here so all interview state changes share one transaction model.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/aiworkflow"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5"
	"github.com/zeromicro/go-zero/core/logx"
)

var interviewStatuses = map[string]struct{}{
	"preparing": {}, "draft": {}, "active": {}, "completed": {}, "abandoned": {}, "failed": {},
}

var interviewSorts = map[string]string{
	"updated_at_desc": "session.updated_at DESC, session.id DESC",
	"updated_at_asc":  "session.updated_at ASC, session.id ASC",
	"created_at_desc": "session.created_at DESC, session.id DESC",
}

var supportedPrimaryLanguages = map[string]string{
	"java":   "Java",
	"go":     "Go",
	"c++":    "C++",
	"python": "Python",
	"rust":   "Rust",
}

type interviewSummaryScanner interface {
	Scan(dest ...any) error
}

const interviewSummarySelect = `
	SELECT session.id::text,
	       session.title,
	       session.status::text,
	       session.primary_language,
	       session.target_company,
	       COALESCE(qset.target_role, ''),
	       resume.id::text,
	       resume.title,
	       count(turn.id)::int,
	       count(turn.id) FILTER (WHERE turn.answer IS NOT NULL)::int,
	       count(turn.id) FILTER (WHERE turn.answer IS NULL AND turn.skipped_at IS NOT NULL)::int,
	       session.current_ordinal,
	       CASE WHEN report.status IN ('completed', 'degraded') THEN report.overall_score::int END,
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
	var startedAt, completedAt *time.Time
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&item.Id,
		&item.Title,
		&item.Status,
		&item.PrimaryLanguage,
		&item.TargetCompany,
		&item.TargetRole,
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
	var operationType, operationStatus, errorCode, errorSummary string
	err = svcCtx.Database.QueryRow(ctx, `
		SELECT session.question_duration_seconds, session.phase, session.agent_mode,
		       session.policy_version, COALESCE(session.completion_reason,''),
		       COALESCE(operation.task_type,''), COALESCE(operation.status::text,''),
		       COALESCE(operation.error_code,''), COALESCE(operation.error_summary,'')
		FROM interview_sessions AS session
		LEFT JOIN LATERAL (
			SELECT task_type,status,error_code,error_summary
			FROM async_tasks
			WHERE user_id=session.user_id AND ref_id=session.id
			  AND task_type IN ('interview.prepare','interview.next_turn')
			ORDER BY created_at DESC,id DESC LIMIT 1
		) operation ON true
		WHERE session.id = $1 AND session.user_id = $2`,
		sessionID,
		userID,
	).Scan(
		&response.QuestionDurationSeconds, &response.Phase, &response.AgentMode,
		&response.PolicyVersion, &response.CompletionReason,
		&operationType, &operationStatus, &errorCode, &errorSummary,
	)
	if err != nil {
		return nil, err
	}
	if operationType != "" && operationStatus != "succeeded" {
		response.Operation = &types.InterviewOperationResponse{
			Type: operationType, Status: operationStatus, ErrorCode: errorCode,
			ErrorSummary: errorSummary, Retryable: operationStatus == "failed",
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
		       END,
		       COALESCE(turn_kind, 'main'),
		       COALESCE(capability_key, ''),
		       COALESCE(parent_turn_id::text, ''),
		       COALESCE(intent,''), COALESCE(difficulty,'medium'), COALESCE(generation_mode,'legacy')
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
			&item.TurnKind,
			&item.CapabilityKey,
			&item.ParentTurnId,
			&item.Intent,
			&item.Difficulty,
			&item.GenerationMode,
		); err != nil {
			return nil, err
		}
		item.StartedAt = formatOptionalTime(startedAt)
		item.AnsweredAt = formatOptionalTime(answeredAt)
		item.SkippedAt = formatOptionalTime(skippedAt)
		response.Turns = append(response.Turns, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if response.Status == "preparing" {
		recoverInterviewPreparation(ctx, svcCtx, userID, sessionID)
	}
	if response.AgentMode == "legacy" && response.Status == "active" && response.CurrentOrdinal > 0 {
		var current *types.InterviewTurnResponse
		for index := range response.Turns {
			if response.Turns[index].Ordinal == response.CurrentOrdinal {
				current = &response.Turns[index]
				break
			}
		}
		if current != nil && current.Answer != "" && current.AnsweredAt != "" {
			answeredAt, parseErr := time.Parse(time.RFC3339Nano, current.AnsweredAt)
			if parseErr == nil && time.Since(answeredAt) >= 15*time.Second {
				shouldFinish, advanceErr := appendNextMainTurn(ctx, svcCtx, userID, sessionID, response.CurrentOrdinal)
				if advanceErr == nil {
					if shouldFinish {
						return completeInterview(ctx, svcCtx, userID, sessionID, false)
					}
					return loadInterview(ctx, svcCtx, userID, sessionID)
				}
			}
		}
	}
	return response, nil
}

func prepareInterview(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.CreateInterviewRequest,
) (*types.InterviewSessionResponse, error) {
	if err := validateID("resume_id", req.ResumeId); err != nil {
		return nil, err
	}
	primaryLanguage, err := validateOptionalText("primary_language", req.PrimaryLanguage, 40)
	if err != nil {
		return nil, err
	}
	if primaryLanguage == "" {
		return nil, apperror.Validation(map[string][]string{"primary_language": {"不能为空"}})
	}
	canonicalLanguage, supported := supportedPrimaryLanguages[strings.ToLower(primaryLanguage)]
	if !supported {
		return nil, apperror.Validation(map[string][]string{
			"primary_language": {"当前仅支持 Java、Go、C++、Python 或 Rust"},
		})
	}
	primaryLanguage = canonicalLanguage
	targetCompany, err := validateOptionalText("target_company", req.TargetCompany, 120)
	if err != nil {
		return nil, err
	}
	if targetCompany == "" {
		return nil, apperror.Validation(map[string][]string{"target_company": {"不能为空"}})
	}
	const targetRole = "backend_development"
	title := fmt.Sprintf("%s · %s 后端模拟面试", targetCompany, primaryLanguage)
	duration := req.QuestionDurationSeconds
	if duration == 0 {
		duration = 180
	}
	if duration != 180 {
		return nil, apperror.Validation(map[string][]string{"question_duration_seconds": {"当前仅支持 180 秒"}})
	}

	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var versionID, resumeStatus string
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(current_version_id::text, ''), status::text
		FROM resumes WHERE id=$1 AND user_id=$2 FOR UPDATE`, req.ResumeId, userID,
	).Scan(&versionID, &resumeStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("RESUME_NOT_FOUND", "未找到该简历", err)
	}
	if err != nil {
		return nil, err
	}
	if resumeStatus != "completed" || versionID == "" {
		return nil, conflict("RESUME_NOT_PARSED", "简历解析完成后才能开始面试", nil)
	}

	questionSetID := uuid.NewString()
	sessionID := uuid.NewString()
	taskID := uuid.NewString()
	inputHash := aiworkflow.InputHash(versionID, strings.ToLower(primaryLanguage), strings.ToLower(targetCompany), targetRole, aiworkflow.BlueprintV2Version)
	_, err = tx.Exec(ctx, `
		INSERT INTO question_sets (
			id, user_id, resume_id, resume_version_id, target_role, status, input_hash, prompt_version,
			primary_language, target_company
		) VALUES ($1,$2,$3,$4,$5,'generating',$6,$7,$8,$9)`,
		questionSetID, userID, req.ResumeId, versionID, targetRole, inputHash, aiworkflow.BlueprintV2Version, primaryLanguage, targetCompany)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO interview_sessions (
			id, user_id, resume_id, resume_version_id, question_set_id, title, status, current_ordinal,
			question_duration_seconds, primary_language, target_company, agent_mode, phase,
			policy_version, interviewer_prompt_version, min_turns, target_turns, max_turns,
			time_budget_minutes, max_follow_up_depth, max_follow_ups_total
		) VALUES ($1,$2,$3,$4,$5,$6,'preparing',0,$7,$8,$9,$10,'preparing',
		          'standard-v2',$11,$12,$13,$14,$15,$16,$17)`,
		sessionID, userID, req.ResumeId, versionID, questionSetID, title, duration, primaryLanguage, targetCompany,
		interviewAgentMode(svcCtx), aiworkflow.NextTurnV2Version,
		contract.StandardMinTurns, contract.StandardTargetTurns, contract.StandardMaxTurns,
		contract.StandardTimeBudgetMinutes, contract.StandardMaxFollowUpDepth, contract.StandardMaxFollowUpsTotal)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO async_tasks (id, user_id, task_type, ref_id, status, progress)
		VALUES ($1,$2,'interview.prepare',$3,'pending',0)`, taskID, userID, sessionID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	queued, err := sharedtasks.NewInterviewPrepareTask(sharedtasks.QuestionGeneratePayload{
		TaskID: taskID, QuestionSetID: questionSetID, SessionID: sessionID,
		UserID: userID, ResumeID: req.ResumeId, ResumeVersionID: versionID, PrimaryLanguage: primaryLanguage,
		TargetCompany: targetCompany, TargetRole: targetRole,
	})
	if err != nil {
		return nil, err
	}
	if _, err := svcCtx.TaskClient.EnqueueContext(ctx, queued, asynq.Queue("heavy"), asynq.Unique(10*time.Minute)); err != nil {
		_, _ = svcCtx.Database.Exec(ctx, `
			UPDATE async_tasks SET status='failed', error_code='TASK_ENQUEUE_FAILED',
			 error_summary='面试准备任务暂时无法启动', error_message=$2,
			 completed_at=now(), updated_at=now() WHERE id=$1`, taskID, err.Error())
		_, _ = svcCtx.Database.Exec(ctx, `UPDATE question_sets SET status='failed', updated_at=now() WHERE id=$1`, questionSetID)
		_, _ = svcCtx.Database.Exec(ctx, `UPDATE interview_sessions SET status='failed', updated_at=now() WHERE id=$1`, sessionID)
		return nil, apperror.Unavailable("面试准备任务暂时无法启动，请重试", nil, err)
	}
	return loadInterview(ctx, svcCtx, userID, sessionID)
}

func recoverInterviewPreparation(ctx context.Context, svcCtx *svc.ServiceContext, userID, sessionID string) {
	var taskID, questionSetID, resumeID, resumeVersionID, primaryLanguage, targetCompany, targetRole string
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT task.id::text, qset.id::text, session.resume_id::text, session.resume_version_id::text,
		       session.primary_language, session.target_company,
		       COALESCE(qset.target_role, 'backend_development')
		FROM interview_sessions AS session
		JOIN question_sets AS qset ON qset.id = session.question_set_id
		JOIN LATERAL (
			SELECT id
			FROM async_tasks
			WHERE user_id = session.user_id
			  AND ref_id = session.id
			  AND task_type = 'interview.prepare'
			  AND status = 'pending'
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) AS task ON true
		WHERE session.id = $1 AND session.user_id = $2
		  AND session.status = 'preparing' AND qset.status = 'generating'`,
		sessionID, userID,
	).Scan(&taskID, &questionSetID, &resumeID, &resumeVersionID, &primaryLanguage, &targetCompany, &targetRole)
	if err != nil {
		return
	}
	queued, err := sharedtasks.NewInterviewPrepareTask(sharedtasks.QuestionGeneratePayload{
		TaskID: taskID, QuestionSetID: questionSetID, SessionID: sessionID,
		UserID: userID, ResumeID: resumeID, ResumeVersionID: resumeVersionID, PrimaryLanguage: primaryLanguage,
		TargetCompany: targetCompany, TargetRole: targetRole,
	})
	if err != nil {
		return
	}
	_, err = svcCtx.TaskClient.EnqueueContext(ctx, queued, asynq.Queue("heavy"), asynq.Unique(10*time.Minute))
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
		return
	}
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

func interviewAgentMode(svcCtx *svc.ServiceContext) string {
	if svcCtx.ChatModel == nil {
		return "rule"
	}
	return "ai"
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
	var mode string
	err = svcCtx.Database.QueryRow(ctx, `SELECT agent_mode FROM interview_sessions WHERE id=$1 AND user_id=$2`, sessionID, userID).Scan(&mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_NOT_FOUND", "未找到该面试", err)
	}
	if err != nil {
		return nil, err
	}
	if mode == "legacy" {
		return saveLegacyInterviewAnswer(ctx, svcCtx, userID, sessionID, ordinal, answer)
	}
	return acceptNextTurnInput(ctx, svcCtx, userID, sessionID, ordinal, answer, false)
}

func skipInterviewTurn(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
	ordinal int,
) (*types.InterviewSessionResponse, error) {
	var mode string
	err := svcCtx.Database.QueryRow(ctx, `SELECT agent_mode FROM interview_sessions WHERE id=$1 AND user_id=$2`, sessionID, userID).Scan(&mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_NOT_FOUND", "未找到该面试", err)
	}
	if err != nil {
		return nil, err
	}
	if mode == "legacy" {
		return skipLegacyInterviewTurn(ctx, svcCtx, userID, sessionID, ordinal)
	}
	return acceptNextTurnInput(ctx, svcCtx, userID, sessionID, ordinal, "", true)
}

func acceptNextTurnInput(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
	ordinal int,
	answer string,
	skip bool,
) (*types.InterviewSessionResponse, error) {
	if ordinal < 1 {
		return nil, apperror.Validation(map[string][]string{"ordinal": {"必须大于等于 1"}})
	}
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var status, phase string
	var currentOrdinal int
	err = tx.QueryRow(ctx, `
		SELECT status::text,phase,current_ordinal
		FROM interview_sessions WHERE id=$1 AND user_id=$2 FOR UPDATE`, sessionID, userID,
	).Scan(&status, &phase, &currentOrdinal)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_NOT_FOUND", "未找到该面试", err)
	}
	if err != nil {
		return nil, err
	}
	if status != "active" {
		return nil, conflict("INTERVIEW_NOT_ACTIVE", "当前面试状态不允许作答", nil)
	}
	if currentOrdinal != ordinal {
		return nil, conflict("INTERVIEW_TURN_NOT_CURRENT", "只能回答当前问题", map[string]any{"current_ordinal": currentOrdinal})
	}
	var turnID, existingAnswer string
	var skippedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT id::text,COALESCE(answer,''),skipped_at
		FROM interview_turns WHERE session_id=$1 AND ordinal=$2 FOR UPDATE`, sessionID, ordinal,
	).Scan(&turnID, &existingAnswer, &skippedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_TURN_NOT_FOUND", "未找到该面试题", err)
	}
	if err != nil {
		return nil, err
	}
	if existingAnswer != "" && (skip || existingAnswer != answer) {
		return nil, conflict("INTERVIEW_TURN_ALREADY_ANSWERED", "该问题已提交不同答案，不能覆盖", nil)
	}
	if skippedAt != nil && !skip {
		return nil, conflict("INTERVIEW_TURN_ALREADY_SKIPPED", "该问题已经跳过，不能重新作答", nil)
	}
	if phase == "deciding" || phase == "decision_failed" {
		if (skip && skippedAt != nil) || (!skip && existingAnswer == answer) {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return loadInterview(ctx, svcCtx, userID, sessionID)
		}
		return nil, conflict("INTERVIEW_DECISION_IN_PROGRESS", "系统正在决定下一轮问题", nil)
	}
	if phase != "answering" {
		return nil, conflict("INTERVIEW_NOT_ANSWERING", "当前不在作答阶段", nil)
	}
	if skip {
		_, err = tx.Exec(ctx, `
			UPDATE interview_turns
			SET started_at=COALESCE(started_at,now()), skipped_at=COALESCE(skipped_at,now()),
			    time_spent_seconds=GREATEST(time_spent_seconds,FLOOR(EXTRACT(EPOCH FROM (now()-COALESCE(started_at,now()))))::int)
			WHERE id=$1 AND answer IS NULL`, turnID)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE interview_turns
			SET started_at=COALESCE(started_at,now()),answer=$2,answered_at=COALESCE(answered_at,now()),skipped_at=NULL,
			    time_spent_seconds=GREATEST(time_spent_seconds,FLOOR(EXTRACT(EPOCH FROM (now()-COALESCE(started_at,now()))))::int)
			WHERE id=$1 AND answer IS NULL AND skipped_at IS NULL`, turnID, answer)
	}
	if err != nil {
		return nil, err
	}
	decisionID := uuid.NewString()
	taskID := uuid.NewString()
	inputHash := aiworkflow.InputHash(sessionID, turnID, answer, fmt.Sprint(skip), fmt.Sprint(ordinal))
	if _, err := tx.Exec(ctx, `
		INSERT INTO interview_turn_decisions (id,session_id,answered_turn_id,user_id,status,input_hash)
		VALUES ($1,$2,$3,$4,'pending',$5)`, decisionID, sessionID, turnID, userID, inputHash); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO async_tasks (id,user_id,task_type,ref_id,status,progress)
		VALUES ($1,$2,'interview.next_turn',$3,'pending',0)`, taskID, userID, sessionID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE interview_sessions SET phase='deciding',updated_at=now() WHERE id=$1`, sessionID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	payload := sharedtasks.InterviewNextTurnPayload{
		TaskID: taskID, DecisionID: decisionID, SessionID: sessionID,
		AnsweredTurnID: turnID, UserID: userID,
	}
	queued, err := sharedtasks.NewInterviewNextTurnTask(payload)
	if err == nil {
		_, err = svcCtx.TaskClient.EnqueueContext(ctx, queued, asynq.Queue("default"), asynq.Unique(10*time.Minute))
	}
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
		compensateNextTurnEnqueue(ctx, svcCtx, payload, err)
		return nil, apperror.Unavailable("下一轮问题暂时无法生成，请重试", nil, err)
	}
	return loadInterview(ctx, svcCtx, userID, sessionID)
}

func compensateNextTurnEnqueue(ctx context.Context, svcCtx *svc.ServiceContext, payload sharedtasks.InterviewNextTurnPayload, cause error) {
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	_, _ = tx.Exec(ctx, `
		UPDATE async_tasks SET status='failed',error_code='TASK_ENQUEUE_FAILED',
		    error_summary='下一轮问题暂时无法生成',error_message=$2,completed_at=now(),updated_at=now()
		WHERE id=$1 AND user_id=$3 AND status='pending'`, payload.TaskID, cause.Error(), payload.UserID)
	_, _ = tx.Exec(ctx, `
		UPDATE interview_turn_decisions SET status='failed',error_code='TASK_ENQUEUE_FAILED',
		    error_summary='下一轮问题暂时无法生成',completed_at=now(),updated_at=now()
		WHERE id=$1 AND user_id=$2 AND status='pending'`, payload.DecisionID, payload.UserID)
	_, _ = tx.Exec(ctx, `
		UPDATE interview_sessions SET phase='decision_failed',updated_at=now()
		WHERE id=$1 AND user_id=$2 AND status='active' AND phase='deciding'`, payload.SessionID, payload.UserID)
	_ = tx.Commit(ctx)
}

func saveLegacyInterviewAnswer(
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
	_, currentOrdinal, err := lockActiveInterview(ctx, tx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	if currentOrdinal != ordinal {
		return nil, conflict("INTERVIEW_TURN_NOT_CURRENT", "只能回答当前问题", map[string]any{"current_ordinal": currentOrdinal})
	}
	var turnID, existingAnswer string
	err = tx.QueryRow(ctx, `
		SELECT id::text, COALESCE(answer, '')
		FROM interview_turns
		WHERE session_id = $1 AND ordinal = $2
		FOR UPDATE`,
		sessionID,
		ordinal,
	).Scan(&turnID, &existingAnswer)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_TURN_NOT_FOUND", "未找到该面试题", err)
	}
	if err != nil {
		return nil, err
	}
	if existingAnswer != "" && existingAnswer != answer {
		return nil, conflict("INTERVIEW_TURN_ALREADY_ANSWERED", "该问题已提交不同答案，不能重复覆盖", nil)
	}
	answerAlreadyStored := existingAnswer != ""
	if existingAnswer == "" {
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
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if answerAlreadyStored {
		// A retried request with the exact same answer is idempotent. The first
		// request owns the decision; if it crashed, loadInterview's stale-answer
		// recovery will advance deterministically after the grace period.
		return loadInterview(ctx, svcCtx, userID, sessionID)
	}
	shouldFinish, err := decideAndAppendNextTurn(ctx, svcCtx, userID, sessionID, ordinal)
	if err != nil {
		// The answer is already durable. Use a deterministic transition so a
		// transient model or tool failure cannot leave an active interview stuck.
		shouldFinish, err = appendNextMainTurn(ctx, svcCtx, userID, sessionID, ordinal)
	}
	if err != nil {
		return nil, err
	}
	if shouldFinish {
		return completeInterview(ctx, svcCtx, userID, sessionID, false)
	}
	return loadInterview(ctx, svcCtx, userID, sessionID)
}

func skipLegacyInterviewTurn(
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
	_, currentOrdinal, err := lockActiveInterview(ctx, tx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	if currentOrdinal != ordinal {
		return nil, conflict("INTERVIEW_TURN_NOT_CURRENT", "只能跳过当前问题", map[string]any{"current_ordinal": currentOrdinal})
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
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	shouldFinish, err := appendNextMainTurn(ctx, svcCtx, userID, sessionID, ordinal)
	if err != nil {
		return nil, err
	}
	if shouldFinish {
		return completeInterview(ctx, svcCtx, userID, sessionID, true)
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
	var status, phase, agentMode string
	err = tx.QueryRow(ctx, `
		SELECT status::text,phase,agent_mode
		FROM interview_sessions
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`,
		sessionID,
		userID,
	).Scan(&status, &phase, &agentMode)
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
	if agentMode != "legacy" && phase != "answering" {
		return nil, conflict("INTERVIEW_DECISION_IN_PROGRESS", "下一轮决策完成后才能结束面试", map[string]any{"phase": phase})
	}
	rows, err := tx.Query(ctx, `
		SELECT ordinal
		FROM interview_turns
		WHERE session_id = $1 AND answer IS NULL AND skipped_at IS NULL
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
		    phase = 'completed',
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
	_, _, _ = ensureReportGeneration(ctx, svcCtx, userID, sessionID)
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
	return prepareInterview(l.ctx, l.svcCtx, userID, req)
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

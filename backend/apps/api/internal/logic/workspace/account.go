package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	"github.com/minio/minio-go/v7"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

type ExportMeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExportMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportMeLogic {
	return &ExportMeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ExportMeLogic) ExportMe() (*types.AccountExportResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	user, err := loadUser(l.ctx, l.svcCtx, userID)
	if err != nil {
		return nil, err
	}
	response := &types.AccountExportResponse{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		User:       *user,
	}
	response.Resumes = queryJSONAgg(l.ctx, l.svcCtx, `
		SELECT COALESCE(json_agg(row_to_json(item) ORDER BY item.updated_at DESC), '[]'::json)
		FROM (
			SELECT resume.id, resume.title, resume.status::text, resume.created_at, resume.updated_at,
			       version.original_filename, left(COALESCE(version.extracted_text, ''), 20000) AS extracted_text
			FROM resumes AS resume
			LEFT JOIN resume_versions AS version ON version.id = resume.current_version_id
			WHERE resume.user_id = $1
		) AS item`, userID)
	response.Jobs = queryJSONAgg(l.ctx, l.svcCtx, `
		SELECT COALESCE(json_agg(row_to_json(item) ORDER BY item.updated_at DESC), '[]'::json)
		FROM (
			SELECT id, company, title, content, extracted_capabilities, created_at, updated_at
			FROM job_descriptions WHERE user_id = $1
		) AS item`, userID)
	response.QuestionSets = queryJSONAgg(l.ctx, l.svcCtx, `
		SELECT COALESCE(json_agg(row_to_json(item) ORDER BY item.created_at DESC), '[]'::json)
		FROM (
			SELECT id, resume_id, target_role, status::text, created_at FROM question_sets WHERE user_id = $1
		) AS item`, userID)
	response.Interviews = queryJSONAgg(l.ctx, l.svcCtx, `
		SELECT COALESCE(json_agg(row_to_json(item) ORDER BY item.updated_at DESC), '[]'::json)
		FROM (
			SELECT session.id, session.title, session.status::text, session.created_at, session.updated_at,
			       COALESCE((
			           SELECT json_agg(json_build_object('ordinal', turn.ordinal, 'question', turn.question, 'answer', turn.answer, 'turn_kind', turn.turn_kind) ORDER BY turn.ordinal)
			           FROM interview_turns AS turn WHERE turn.session_id = session.id
			       ), '[]'::json) AS turns
			FROM interview_sessions AS session WHERE session.user_id = $1
		) AS item`, userID)
	response.Reports = queryJSONAgg(l.ctx, l.svcCtx, `
		SELECT COALESCE(json_agg(row_to_json(item) ORDER BY item.updated_at DESC), '[]'::json)
		FROM (
			SELECT report.id, report.session_id, report.status, report.overall_score, report.strengths, report.improvements, report.next_steps, report.updated_at
			FROM interview_reports AS report
			JOIN interview_sessions AS session ON session.id = report.session_id
			WHERE session.user_id = $1
		) AS item`, userID)
	response.Tasks = queryJSONAgg(l.ctx, l.svcCtx, `
		SELECT COALESCE(json_agg(row_to_json(item) ORDER BY item.created_at DESC), '[]'::json)
		FROM (
			SELECT id, task_type, status::text, progress, error_code, created_at, completed_at
			FROM async_tasks WHERE user_id = $1
		) AS item`, userID)
	response.SkillProfile = queryJSONAgg(l.ctx, l.svcCtx, `
		SELECT COALESCE(json_agg(row_to_json(item)), '[]'::json)
		FROM (
			SELECT strengths, gaps, notes, source_session_id, updated_at
			FROM user_skill_profiles WHERE user_id = $1
		) AS item`, userID)
	response.ModelInvocations = queryJSONAgg(l.ctx, l.svcCtx, `
		SELECT COALESCE(json_agg(row_to_json(item) ORDER BY item.created_at DESC), '[]'::json)
		FROM (
			SELECT id, provider, model, prompt_key, prompt_version, status, total_tokens, estimated_cost_micros, latency_ms, error_code, created_at
			FROM model_invocations WHERE user_id = $1
			LIMIT 500
		) AS item`, userID)
	return response, nil
}

func queryJSONAgg(ctx context.Context, svcCtx *svc.ServiceContext, sql, userID string) any {
	var raw []byte
	if err := svcCtx.Database.QueryRow(ctx, sql, userID).Scan(&raw); err != nil || len(raw) == 0 {
		return []any{}
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return []any{}
	}
	return value
}

type DeleteMeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMeLogic {
	return &DeleteMeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeleteMeLogic) DeleteMe(req *types.DeleteAccountRequest) error {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return err
	}
	var passwordHash string
	if err := l.svcCtx.Database.QueryRow(l.ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&passwordHash); err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		return apperror.New("CURRENT_PASSWORD_INCORRECT", "当前密码不正确", http.StatusBadRequest, nil, nil)
	}
	rows, err := l.svcCtx.Database.Query(l.ctx, `
		SELECT version.object_key
		FROM resume_versions AS version
		JOIN resumes AS resume ON resume.id = version.resume_id
		WHERE resume.user_id = $1`, userID)
	if err != nil {
		return err
	}
	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err == nil && key != "" {
			keys = append(keys, key)
		}
	}
	rows.Close()
	for _, key := range keys {
		_ = l.svcCtx.ObjectStore.RemoveObject(l.ctx, l.svcCtx.Config.Runtime.ObjectStore.Bucket, key, minio.RemoveObjectOptions{})
	}
	if _, err := l.svcCtx.Database.Exec(l.ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		return err
	}
	return nil
}

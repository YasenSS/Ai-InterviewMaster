package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/jackc/pgx/v5"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetInterviewReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetInterviewReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetInterviewReportLogic {
	return &GetInterviewReportLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetInterviewReportLogic) GetInterviewReport(req *types.InterviewPath) (*types.InterviewReportResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateID("id", req.Id); err != nil {
		return nil, err
	}
	var status string
	err = l.svcCtx.Database.QueryRow(l.ctx, `
		SELECT status::text FROM interview_sessions WHERE id=$1 AND user_id=$2`,
		req.Id, userID,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("INTERVIEW_NOT_FOUND", "未找到该面试", err)
	}
	if err != nil {
		return nil, err
	}
	if status != "completed" {
		return nil, conflict("INTERVIEW_NOT_COMPLETED", "面试完成后才能生成报告", nil)
	}
	reportID, _, err := ensureReportGeneration(l.ctx, l.svcCtx, userID, req.Id)
	if err != nil {
		return nil, err
	}
	return loadInterviewReport(l.ctx, l.svcCtx, userID, req.Id, reportID)
}

func loadInterviewReport(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID, reportID string,
) (*types.InterviewReportResponse, error) {
	response := &types.InterviewReportResponse{
		Turns:        []types.TurnReportResponse{},
		Strengths:    []string{},
		Improvements: []string{},
		NextSteps:    []string{},
	}
	var strengthsRaw, improvementsRaw, nextStepsRaw, qualityRaw []byte
	var createdAt, updatedAt time.Time
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT report.id::text,
		       report.session_id::text,
		       report.overall_score,
		       report.strengths,
		       report.improvements,
		       report.next_steps,
		       report.quality_gate,
		       report.status,
		       report.degraded,
		       COALESCE(report.error_code, ''),
		       COALESCE(report.error_summary, ''),
		       report.created_at,
		       report.updated_at
		FROM interview_reports AS report
		JOIN interview_sessions AS session ON session.id = report.session_id
		WHERE report.id = $1
		  AND report.session_id = $2
		  AND session.user_id = $3`,
		reportID, sessionID, userID,
	).Scan(
		&response.Id,
		&response.SessionId,
		&response.OverallScore,
		&strengthsRaw,
		&improvementsRaw,
		&nextStepsRaw,
		&qualityRaw,
		&response.Status,
		&response.Degraded,
		&response.ErrorCode,
		&response.ErrorSummary,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	response.CreatedAt = formatTime(createdAt)
	response.UpdatedAt = formatTime(updatedAt)
	_ = json.Unmarshal(strengthsRaw, &response.Strengths)
	_ = json.Unmarshal(improvementsRaw, &response.Improvements)
	_ = json.Unmarshal(nextStepsRaw, &response.NextSteps)
	var quality map[string]any
	_ = json.Unmarshal(qualityRaw, &quality)
	response.QualityPassed, _ = quality["passed"].(bool)
	var operationStatus, operationCode, operationSummary string
	_ = svcCtx.Database.QueryRow(ctx, `
		SELECT status::text,COALESCE(error_code,''),COALESCE(error_summary,'') FROM async_tasks
		WHERE user_id=$1 AND ref_id=$2::uuid AND task_type='report.generate'
		ORDER BY created_at DESC LIMIT 1`, userID, sessionID,
	).Scan(&operationStatus, &operationCode, &operationSummary)
	if operationStatus != "" && operationStatus != "succeeded" {
		response.Operation = &types.InterviewOperationResponse{
			Type: "report.generate", Status: operationStatus, ErrorCode: operationCode,
			ErrorSummary: operationSummary, Retryable: operationStatus == "failed",
		}
	}
	if response.Status == "pending" || response.Status == "running" || response.Status == "failed" {
		return response, nil
	}
	rows, err := svcCtx.Database.Query(ctx, `
		SELECT turn.ordinal, turn.question, COALESCE(turn.answer, ''), turn_report.score,
		       turn_report.critique, turn_report.golden_answer, turn_report.evidence
		FROM interview_turn_reports AS turn_report
		JOIN interview_turns AS turn ON turn.id = turn_report.turn_id
		WHERE turn_report.report_id = $1
		ORDER BY turn.ordinal ASC`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item types.TurnReportResponse
		var evidenceRaw []byte
		if err := rows.Scan(&item.Ordinal, &item.Question, &item.Answer, &item.Score, &item.Critique, &item.GoldenAnswer, &evidenceRaw); err != nil {
			return nil, err
		}
		item.Evidence = []string{}
		_ = json.Unmarshal(evidenceRaw, &item.Evidence)
		response.Turns = append(response.Turns, item)
	}
	return response, rows.Err()
}

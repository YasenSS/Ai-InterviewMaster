// Report generation and loading are kept together to enforce single-report reuse.
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
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
	tx, err := l.svcCtx.Database.Begin(l.ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(l.ctx)
	var status string
	err = tx.QueryRow(l.ctx, `
		SELECT status::text
		FROM interview_sessions
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`,
		req.Id,
		userID,
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
	var reportID string
	err = tx.QueryRow(
		l.ctx,
		`SELECT id::text FROM interview_reports WHERE session_id = $1`,
		req.Id,
	).Scan(&reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		reportID, err = generateInterviewReport(l.ctx, tx, req.Id)
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(l.ctx); err != nil {
		return nil, err
	}
	return loadInterviewReport(l.ctx, l.svcCtx, userID, req.Id, reportID)
}

type reportTurn struct {
	ID       string
	Ordinal  int
	Question string
	Answer   string
	Score    int
}

func generateInterviewReport(ctx context.Context, tx pgx.Tx, sessionID string) (string, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, ordinal, question, COALESCE(answer, '')
		FROM interview_turns
		WHERE session_id = $1
		ORDER BY ordinal ASC`,
		sessionID,
	)
	if err != nil {
		return "", err
	}
	turns := []reportTurn{}
	total := 0
	answered := 0
	for rows.Next() {
		var turn reportTurn
		if err := rows.Scan(&turn.ID, &turn.Ordinal, &turn.Question, &turn.Answer); err != nil {
			rows.Close()
			return "", err
		}
		turn.Score = scoreAnswer(turn.Answer)
		if strings.TrimSpace(turn.Answer) != "" {
			answered++
		}
		total += turn.Score
		turns = append(turns, turn)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(turns) == 0 {
		return "", conflict("INTERVIEW_HAS_NO_TURNS", "该面试没有可生成报告的题目", nil)
	}
	overallScore := total / len(turns)
	strengths := []string{}
	if answered > 0 {
		strengths = append(strengths, "能够结合个人经历作答")
	}
	if answered == len(turns) {
		strengths = append(strengths, "完成了全部面试题")
	}
	if strengths == nil {
		strengths = []string{}
	}
	improvements := []string{"量化结果", "技术取舍"}
	if answered < len(turns) {
		improvements = append(improvements, "回答完整度")
	}
	nextSteps := []string{"按 STAR 结构重答低分题", "为关键成果补充可验证数据"}
	qualityPassed := answered == len(turns) && overallScore >= 60
	qualityGate, _ := json.Marshal(map[string]any{
		"passed":         qualityPassed,
		"answered_turns": answered,
		"total_turns":    len(turns),
		"minimum_score":  60,
	})
	reportID := uuid.NewString()
	_, err = tx.Exec(ctx, `
		INSERT INTO interview_reports (
			id,
			session_id,
			overall_score,
			strengths,
			improvements,
			next_steps,
			quality_gate
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		reportID,
		sessionID,
		overallScore,
		encodeStrings(strengths),
		encodeStrings(improvements),
		encodeStrings(nextSteps),
		qualityGate,
	)
	if err != nil {
		return "", err
	}
	for _, turn := range turns {
		critique := "回答方向正确；建议补充量化指标、约束条件和个人贡献。"
		evidence := []string{"用户提供了回答内容"}
		if strings.TrimSpace(turn.Answer) == "" {
			critique = "该题未作答，无法评估相关能力；建议补充完整回答。"
			evidence = []string{}
		}
		goldenAnswer := "建议按 STAR 展开：交代背景与目标，说明关键行动、技术取舍和可验证结果。"
		_, err = tx.Exec(ctx, `
			INSERT INTO interview_turn_reports (
				report_id, turn_id, score, critique, golden_answer, evidence
			)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			reportID,
			turn.ID,
			turn.Score,
			critique,
			goldenAnswer,
			encodeStrings(evidence),
		)
		if err != nil {
			return "", err
		}
	}
	return reportID, nil
}

func scoreAnswer(answer string) int {
	length := len([]rune(strings.TrimSpace(answer)))
	if length == 0 {
		return 0
	}
	score := 40 + length/3
	if score > 95 {
		score = 95
	}
	return score
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
		       report.created_at,
		       report.updated_at
		FROM interview_reports AS report
		JOIN interview_sessions AS session ON session.id = report.session_id
		WHERE report.id = $1
		  AND report.session_id = $2
		  AND session.user_id = $3`,
		reportID,
		sessionID,
		userID,
	).Scan(
		&response.Id,
		&response.SessionId,
		&response.OverallScore,
		&strengthsRaw,
		&improvementsRaw,
		&nextStepsRaw,
		&qualityRaw,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	response.Status = "completed"
	response.CreatedAt = formatTime(createdAt)
	response.UpdatedAt = formatTime(updatedAt)
	_ = json.Unmarshal(strengthsRaw, &response.Strengths)
	_ = json.Unmarshal(improvementsRaw, &response.Improvements)
	_ = json.Unmarshal(nextStepsRaw, &response.NextSteps)
	var quality map[string]any
	_ = json.Unmarshal(qualityRaw, &quality)
	response.QualityPassed, _ = quality["passed"].(bool)
	rows, err := svcCtx.Database.Query(ctx, `
		SELECT turn.ordinal,
		       turn.question,
		       COALESCE(turn.answer, ''),
		       turn_report.score,
		       turn_report.critique,
		       turn_report.golden_answer,
		       turn_report.evidence
		FROM interview_turn_reports AS turn_report
		JOIN interview_turns AS turn ON turn.id = turn_report.turn_id
		WHERE turn_report.report_id = $1
		ORDER BY turn.ordinal ASC`,
		reportID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item types.TurnReportResponse
		var evidenceRaw []byte
		if err := rows.Scan(
			&item.Ordinal,
			&item.Question,
			&item.Answer,
			&item.Score,
			&item.Critique,
			&item.GoldenAnswer,
			&evidenceRaw,
		); err != nil {
			return nil, err
		}
		item.Evidence = []string{}
		_ = json.Unmarshal(evidenceRaw, &item.Evidence)
		response.Turns = append(response.Turns, item)
	}
	return response, rows.Err()
}

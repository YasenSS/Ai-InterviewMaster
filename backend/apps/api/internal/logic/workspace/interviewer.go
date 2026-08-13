package workspace

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/internal/aiworkflow"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
	aitools "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/tools"
	"github.com/jackc/pgx/v5"
)

func blueprintFollowUpBudget(raw []byte) int {
	var blueprint contract.InterviewBlueprint
	if err := json.Unmarshal(raw, &blueprint); err != nil {
		return 0
	}
	if blueprint.FollowUpBudget < 0 {
		return 0
	}
	if blueprint.FollowUpBudget > 5 {
		return 5
	}
	return blueprint.FollowUpBudget
}

func maybeInsertFollowUp(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
	ordinal int,
) {
	if svcCtx == nil || svcCtx.ChatModel == nil {
		return
	}
	var (
		turnID, turnKind, capability, question, answer string
		followUpsUsed, followUpBudget                  int
		blueprintRaw                                   []byte
	)
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT turn.id::text,
		       COALESCE(turn.turn_kind, 'main'),
		       COALESCE(turn.capability_key, ''),
		       turn.question,
		       COALESCE(turn.answer, ''),
		       session.follow_ups_used,
		       session.follow_up_budget,
		       COALESCE(session.blueprint, '{}'::jsonb)
		FROM interview_turns AS turn
		JOIN interview_sessions AS session ON session.id = turn.session_id
		WHERE turn.session_id = $1 AND turn.ordinal = $2 AND session.user_id = $3`,
		sessionID, ordinal, userID,
	).Scan(&turnID, &turnKind, &capability, &question, &answer, &followUpsUsed, &followUpBudget, &blueprintRaw)
	if err != nil || turnKind == "follow_up" || strings.TrimSpace(answer) == "" {
		return
	}
	var alreadyFollowed bool
	_ = svcCtx.Database.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM interview_turns WHERE parent_turn_id = $1::uuid)`,
		turnID,
	).Scan(&alreadyFollowed)
	if alreadyFollowed {
		return
	}
	var blueprint contract.InterviewBlueprint
	_ = json.Unmarshal(blueprintRaw, &blueprint)
	recent := loadRecentTurns(ctx, svcCtx, sessionID, ordinal)
	decision, response, _ := aiworkflow.DecideFollowUp(ctx, svcCtx.ChatModel, aiworkflow.FollowUpInput{
		UserID:            userID,
		SessionID:         sessionID,
		CurrentQuestion:   question,
		CurrentAnswer:     answer,
		CurrentCapability: capability,
		CurrentIsFollowUp: turnKind == "follow_up",
		FollowUpsUsed:     followUpsUsed,
		FollowUpBudget:    followUpBudget,
		RecentTurns:       recent,
		Blueprint:         blueprint,
		Tools: []platformai.Tool{
			aitools.NewResumeFactsTool(svcCtx.Database, userID),
			aitools.NewCompanyIntelTool(svcCtx.Database),
		},
	})
	if decision.Action != contract.ActionFollowUp || strings.TrimSpace(decision.Question) == "" {
		return
	}
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	if err := insertFollowUpTurn(ctx, tx, sessionID, turnID, ordinal, decision, response); err != nil {
		return
	}
	_ = tx.Commit(ctx)
}

func insertFollowUpTurn(
	ctx context.Context,
	tx pgx.Tx,
	sessionID, parentTurnID string,
	afterOrdinal int,
	decision contract.InterviewerDecision,
	response platformai.GenerateResponse,
) error {
	if _, err := tx.Exec(ctx, `
		UPDATE interview_turns
		SET ordinal = ordinal + 1000
		WHERE session_id = $1 AND ordinal > $2`,
		sessionID, afterOrdinal,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO interview_turns (
			session_id, ordinal, question, started_at, turn_kind, parent_turn_id, capability_key
		)
		VALUES ($1, $2, $3, now(), 'follow_up', $4, $5)`,
		sessionID,
		afterOrdinal+1,
		decision.Question,
		parentTurnID,
		decision.CapabilityKey,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE interview_turns
		SET ordinal = ordinal - 999
		WHERE session_id = $1 AND ordinal >= 1000`,
		sessionID,
	); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE interview_sessions
		SET follow_ups_used = follow_ups_used + 1,
		    current_capability_key = $2,
		    interviewer_model = COALESCE(NULLIF($3, ''), interviewer_model),
		    updated_at = now()
		WHERE id = $1`,
		sessionID,
		decision.CapabilityKey,
		response.Model,
	)
	return err
}

func loadRecentTurns(ctx context.Context, svcCtx *svc.ServiceContext, sessionID string, currentOrdinal int) []map[string]any {
	rows, err := svcCtx.Database.Query(ctx, `
		SELECT ordinal, COALESCE(turn_kind, 'main'), question, COALESCE(answer, ''), COALESCE(capability_key, '')
		FROM interview_turns
		WHERE session_id = $1 AND ordinal < $2
		ORDER BY ordinal DESC
		LIMIT 3`,
		sessionID, currentOrdinal,
	)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var ordinal int
		var kind, question, answer, capability string
		if err := rows.Scan(&ordinal, &kind, &question, &answer, &capability); err != nil {
			return result
		}
		result = append(result, map[string]any{
			"ordinal":        ordinal,
			"turn_kind":      kind,
			"question":       platformai.ClipRunes(question, 500),
			"answer":         platformai.ClipRunes(answer, 800),
			"capability_key": capability,
		})
	}
	return result
}

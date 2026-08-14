package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/internal/aiworkflow"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
	aitools "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/tools"
	"github.com/jackc/pgx/v5"
)

const minimumAdaptiveInterviewTurns = 3

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

type answeredTurnContext struct {
	turnID             string
	turnKind           string
	capability         string
	question           string
	answer             string
	primaryLanguage    string
	targetCompany      string
	targetRole         string
	followUpsUsed      int
	followUpBudget     int
	followUpDepth      int
	completedTurns     int
	blueprint          contract.InterviewBlueprint
	allowedEvidenceIDs []string
}

// decideAndAppendNextTurn is the only adaptive transition after an answer. It
// asks the bounded interviewer node for a decision, then applies that decision
// transactionally. The boolean result is true when the interview should end.
func decideAndAppendNextTurn(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
	ordinal int,
) (bool, error) {
	input, err := loadAnsweredTurnContext(ctx, svcCtx, userID, sessionID, ordinal)
	if err != nil {
		return false, err
	}
	decision, response, err := aiworkflow.DecideNextTurn(ctx, svcCtx.ChatModel, aiworkflow.NextTurnInput{
		UserID:                 userID,
		SessionID:              sessionID,
		CurrentQuestion:        input.question,
		CurrentAnswer:          input.answer,
		CurrentCapability:      input.capability,
		PrimaryLanguage:        input.primaryLanguage,
		TargetCompany:          input.targetCompany,
		TargetRole:             input.targetRole,
		CurrentFollowUpDepth:   input.followUpDepth,
		FollowUpsUsed:          input.followUpsUsed,
		FollowUpBudget:         input.followUpBudget,
		MaxFollowUpDepth:       contract.DefaultMaxFollowUpDepth,
		CompletedTurns:         input.completedTurns,
		MinimumTurnsForFinish:  minimumAdaptiveInterviewTurns,
		RecentTurns:            loadRecentTurns(ctx, svcCtx, sessionID, ordinal),
		Blueprint:              input.blueprint,
		AllowedEvidenceFactIDs: input.allowedEvidenceIDs,
		Tools: []platformai.Tool{
			aitools.NewResumeFactsTool(svcCtx.Database, userID),
			aitools.NewCompanyIntelTool(svcCtx.Database),
		},
	})
	if err != nil {
		decision = contract.NextCapabilityDecision(input.capability, input.blueprint.CapabilityKeys, "interviewer decision failed")
	}
	return applyNextTurnDecision(ctx, svcCtx, userID, sessionID, ordinal, input.turnID, decision, response)
}

func loadAnsweredTurnContext(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
	ordinal int,
) (answeredTurnContext, error) {
	var result answeredTurnContext
	var blueprintRaw []byte
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT turn.id::text,
		       COALESCE(turn.turn_kind, 'main'),
		       COALESCE(turn.capability_key, ''),
		       turn.question,
		       COALESCE(turn.answer, ''),
		       session.primary_language,
		       session.target_company,
		       COALESCE(qset.target_role, 'backend_development'),
		       session.follow_ups_used,
		       session.follow_up_budget,
		       COALESCE(session.blueprint, '{}'::jsonb),
		       (SELECT count(*)::int
		        FROM interview_turns AS completed
		        WHERE completed.session_id = session.id
		          AND (completed.answer IS NOT NULL OR completed.skipped_at IS NOT NULL))
		FROM interview_turns AS turn
		JOIN interview_sessions AS session ON session.id = turn.session_id
		LEFT JOIN question_sets AS qset ON qset.id = session.question_set_id
		WHERE turn.session_id = $1
		  AND turn.ordinal = $2
		  AND session.user_id = $3`,
		sessionID, ordinal, userID,
	).Scan(
		&result.turnID,
		&result.turnKind,
		&result.capability,
		&result.question,
		&result.answer,
		&result.primaryLanguage,
		&result.targetCompany,
		&result.targetRole,
		&result.followUpsUsed,
		&result.followUpBudget,
		&blueprintRaw,
		&result.completedTurns,
	)
	if err != nil {
		return result, err
	}
	_ = json.Unmarshal(blueprintRaw, &result.blueprint)
	result.followUpDepth = loadFollowUpDepth(ctx, svcCtx, result.turnID)
	result.allowedEvidenceIDs = loadResumeEvidenceIDs(ctx, svcCtx, sessionID)
	return result, nil
}

func loadFollowUpDepth(ctx context.Context, svcCtx *svc.ServiceContext, turnID string) int {
	var depth int
	err := svcCtx.Database.QueryRow(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT id, parent_turn_id, 0 AS depth
			FROM interview_turns
			WHERE id = $1
			UNION ALL
			SELECT parent.id, parent.parent_turn_id, ancestors.depth + 1
			FROM interview_turns AS parent
			JOIN ancestors ON parent.id = ancestors.parent_turn_id
		)
		SELECT COALESCE(max(depth), 0) FROM ancestors`, turnID,
	).Scan(&depth)
	if err != nil || depth < 0 {
		return 0
	}
	return depth
}

func loadResumeEvidenceIDs(ctx context.Context, svcCtx *svc.ServiceContext, sessionID string) []string {
	rows, err := svcCtx.Database.Query(ctx, `
		SELECT fact.id::text
		FROM resume_facts AS fact
		JOIN interview_sessions AS session ON session.resume_version_id = fact.resume_version_id
		WHERE session.id = $1
		ORDER BY fact.created_at ASC`, sessionID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && strings.TrimSpace(id) != "" {
			result = append(result, id)
		}
	}
	return result
}

func applyNextTurnDecision(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
	afterOrdinal int,
	parentTurnID string,
	decision contract.InterviewerDecision,
	response platformai.GenerateResponse,
) (bool, error) {
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var status, questionSetID string
	var currentOrdinal int
	err = tx.QueryRow(ctx, `
		SELECT status::text, current_ordinal, question_set_id::text
		FROM interview_sessions
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`, sessionID, userID,
	).Scan(&status, &currentOrdinal, &questionSetID)
	if err != nil {
		return false, err
	}
	if status != "active" || currentOrdinal != afterOrdinal {
		return false, tx.Commit(ctx)
	}

	if decision.Action == contract.ActionFinish {
		return true, tx.Commit(ctx)
	}

	nextOrdinal := afterOrdinal + 1
	if decision.Action == contract.ActionFollowUp {
		_, err = tx.Exec(ctx, `
			INSERT INTO interview_turns (
				session_id, ordinal, question, started_at, turn_kind, parent_turn_id, capability_key
			) VALUES ($1, $2, $3, now(), 'follow_up', $4, $5)`,
			sessionID, nextOrdinal, decision.Question, parentTurnID, decision.CapabilityKey,
		)
		if err != nil {
			return false, err
		}
		_, err = tx.Exec(ctx, `
			UPDATE interview_sessions
			SET current_ordinal = $2,
			    current_capability_key = $3,
			    follow_ups_used = follow_ups_used + 1,
			    interviewer_model = COALESCE(NULLIF($4, ''), interviewer_model),
			    updated_at = now()
			WHERE id = $1`,
			sessionID, nextOrdinal, decision.CapabilityKey, response.Model,
		)
		if err != nil {
			return false, err
		}
		return false, tx.Commit(ctx)
	}

	var sourceQuestionID, question, capability string
	err = tx.QueryRow(ctx, `
		SELECT question.id::text, question.question, COALESCE(question.capability_key, '')
		FROM questions AS question
		WHERE question.question_set_id = $1
		  AND NOT EXISTS (
			SELECT 1
			FROM interview_turns AS used
			WHERE used.session_id = $2
			  AND used.source_question_id = question.id
		  )
		ORDER BY CASE WHEN question.capability_key = $3 THEN 0 ELSE 1 END,
		         question.ordinal ASC
		LIMIT 1
		FOR UPDATE`, questionSetID, sessionID, decision.CapabilityKey,
	).Scan(&sourceQuestionID, &question, &capability)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, tx.Commit(ctx)
	}
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO interview_turns (
			session_id, ordinal, question, started_at, turn_kind, capability_key, source_question_id
		) VALUES ($1, $2, $3, now(), 'main', $4, $5)`,
		sessionID, nextOrdinal, question, capability, sourceQuestionID,
	)
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE interview_sessions
		SET current_ordinal = $2,
		    current_capability_key = $3,
		    interviewer_model = COALESCE(NULLIF($4, ''), interviewer_model),
		    updated_at = now()
		WHERE id = $1`,
		sessionID, nextOrdinal, capability, response.Model,
	)
	if err != nil {
		return false, err
	}
	return false, tx.Commit(ctx)
}

// appendNextMainTurn advances without a model decision, for skips and crash
// recovery. It still honours the blueprint order and never exposes the hidden
// candidate set to the client.
func appendNextMainTurn(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, sessionID string,
	afterOrdinal int,
) (bool, error) {
	var capability string
	var blueprintRaw []byte
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT COALESCE(current_capability_key, ''), COALESCE(blueprint, '{}'::jsonb)
		FROM interview_sessions
		WHERE id = $1 AND user_id = $2`, sessionID, userID,
	).Scan(&capability, &blueprintRaw)
	if err != nil {
		return false, err
	}
	var blueprint contract.InterviewBlueprint
	_ = json.Unmarshal(blueprintRaw, &blueprint)
	decision := contract.NextCapabilityDecision(capability, blueprint.CapabilityKeys, "deterministic interview advance")
	return applyNextTurnDecision(ctx, svcCtx, userID, sessionID, afterOrdinal, "", decision, platformai.GenerateResponse{})
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

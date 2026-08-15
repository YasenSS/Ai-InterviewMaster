package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/internal/aiworkflow"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
	aitools "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/tools"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type nextTurnRuntime struct {
	SessionID       string
	UserID          string
	ResumeVersionID string
	QuestionSetID   string
	Language        string
	Company         string
	TargetRole      string
	AgentMode       string
	CurrentOrdinal  int
	FollowUpsUsed   int
	StartedAt       time.Time
	Blueprint       contract.InterviewBlueprintV2
	Progress        map[string]contract.CapabilityProgress
	AnsweredTurnID  string
	TurnKind        string
	Capability      string
	Question        string
	Answer          string
	Skipped         bool
	FollowUpDepth   int
	CompletedTurns  int
	AllowedFactIDs  []string
	RecentTurns     []map[string]any
}

// InterviewNextTurnHandler owns the durable answer -> decision -> next turn
// transition. API retries cannot overwrite an answer and worker replay cannot
// create a second next turn because the decision row is unique per answer.
func InterviewNextTurnHandler(db *pgxpool.Pool, chat platformai.ChatModel) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload sharedtasks.InterviewNextTurnPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("%w: decode next-turn task: %v", asynq.SkipRetry, err)
		}
		if err := claimNextTurn(ctx, db, payload); err != nil {
			if errors.Is(err, errTaskAlreadySucceeded) {
				return nil
			}
			return err
		}
		runtime, err := loadNextTurnRuntime(ctx, db, payload)
		if err != nil {
			return failNextTurn(ctx, db, payload, "INTERVIEW_STATE_INVALID", "无法读取下一轮决策上下文", err)
		}
		if runtime.AgentMode == "ai" && chat == nil {
			return failNextTurn(ctx, db, payload, "AI_NOT_CONFIGURED", "实时面试模型当前不可用，请重试", errors.New("AI session has no configured chat model"))
		}

		var decision contract.NextTurnDecisionV2
		var response platformai.GenerateResponse
		generationMode := runtime.AgentMode
		if generationMode != "ai" {
			generationMode = "fallback"
		}
		if payload.UseFallback || runtime.Skipped || runtime.AgentMode == "rule" {
			reason := "使用显式规则模式推进面试"
			if payload.UseFallback {
				reason = "用户选择使用系统兜底问题继续"
			}
			decision, err = fallbackNextTurnDecision(ctx, db, runtime, reason)
			if err != nil {
				return failNextTurn(ctx, db, payload, "FALLBACK_UNAVAILABLE", "没有可用的安全问题", err)
			}
		} else {
			decision, response, err = aiworkflow.DecideNextTurnV2(ctx, chat, aiworkflow.NextTurnInputV2{
				UserID: payload.UserID, TaskID: payload.TaskID, SessionID: payload.SessionID, DecisionID: payload.DecisionID,
				Language: runtime.Language, Company: runtime.Company, TargetRole: runtime.TargetRole,
				Question: runtime.Question, Answer: runtime.Answer, Capability: runtime.Capability,
				CurrentFollowUpDepth: runtime.FollowUpDepth, FollowUpsUsed: runtime.FollowUpsUsed,
				CompletedTurns: runtime.CompletedTurns, ElapsedMinutes: elapsedMinutes(runtime.StartedAt),
				Blueprint: runtime.Blueprint, CapabilityProgress: runtime.Progress,
				RecentTurns: runtime.RecentTurns, AllowedEvidenceFactIDs: runtime.AllowedFactIDs,
				Tools: []platformai.Tool{
					aitools.NewResumeVersionFactsTool(db, payload.UserID, runtime.ResumeVersionID),
					aitools.NewCompanyIntelTool(db),
				},
			})
			if err != nil {
				return failNextTurn(ctx, db, payload, aiFailureCode(err), "实时面试决策失败，请重试", err)
			}
		}

		progress := updateCapabilityProgress(runtime.Progress, runtime.Capability, decision.CoverageObservation, runtime.TurnKind)
		policy := contract.EvaluateInterviewPolicyV2(runtime.Blueprint, contract.InterviewPolicyStateV2{
			CompletedTurns:              runtime.CompletedTurns,
			ElapsedMinutes:              elapsedMinutes(runtime.StartedAt),
			CoveredWeight:               coveredWeight(runtime.Blueprint, progress),
			CriticalCapabilitiesCovered: criticalCapabilitiesCovered(runtime.Blueprint, progress),
			ModelRecommendedFinish:      decision.Action == contract.ActionRecommendFinish,
		})
		if policy.ShouldFinish {
			return completeNextTurn(ctx, db, payload, runtime, decision, response, progress, policy.Reason)
		}
		if decision.Action == contract.ActionRecommendFinish {
			decision, err = fallbackNextTurnDecision(ctx, db, runtime, "服务端策略要求继续覆盖能力")
			if err != nil {
				return failNextTurn(ctx, db, payload, "FALLBACK_UNAVAILABLE", "面试尚未达到结束条件且没有安全问题", err)
			}
			generationMode = "fallback"
		}
		return appendNextTurn(ctx, db, payload, runtime, decision, response, progress, generationMode)
	}
}

var errTaskAlreadySucceeded = errors.New("next-turn task already succeeded")

func claimNextTurn(ctx context.Context, db *pgxpool.Pool, payload sharedtasks.InterviewNextTurnPayload) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var taskStatus string
	var taskUpdatedAt time.Time
	err = tx.QueryRow(ctx, `SELECT status::text,updated_at FROM async_tasks WHERE id=$1 AND user_id=$2 FOR UPDATE`, payload.TaskID, payload.UserID).Scan(&taskStatus, &taskUpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: next-turn task not found", asynq.SkipRetry)
	}
	if taskStatus == "succeeded" {
		return errTaskAlreadySucceeded
	}
	if taskStatus != "pending" && taskStatus != "running" {
		return fmt.Errorf("%w: next-turn task is not runnable", asynq.SkipRetry)
	}
	if taskStatus == "running" && time.Since(taskUpdatedAt) < 10*time.Minute {
		return errors.New("next-turn task is already running")
	}
	var decisionStatus string
	var decisionUpdatedAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT decision.status,decision.updated_at
		FROM interview_turn_decisions AS decision
		JOIN interview_sessions AS session ON session.id=decision.session_id
		JOIN interview_turns AS turn ON turn.id=decision.answered_turn_id
		WHERE decision.id=$1 AND decision.session_id=$2 AND decision.user_id=$3
		  AND decision.answered_turn_id=$4
		  AND session.user_id=$3 AND session.status='active' AND session.phase IN ('deciding','decision_failed')
		  AND turn.session_id=session.id AND turn.ordinal=session.current_ordinal
		  AND (turn.answer IS NOT NULL OR turn.skipped_at IS NOT NULL)
		FOR UPDATE`, payload.DecisionID, payload.SessionID, payload.UserID, payload.AnsweredTurnID,
	).Scan(&decisionStatus, &decisionUpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: decision is stale or cross-tenant", asynq.SkipRetry)
	}
	if decisionStatus == "succeeded" {
		return errTaskAlreadySucceeded
	}
	if decisionStatus != "pending" && decisionStatus != "running" && decisionStatus != "failed" {
		return fmt.Errorf("%w: decision is not runnable", asynq.SkipRetry)
	}
	if decisionStatus == "running" && time.Since(decisionUpdatedAt) < 10*time.Minute {
		return errors.New("next-turn decision is already running")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE interview_turn_decisions
		SET status='running', attempt=attempt+1, started_at=COALESCE(started_at,now()),
		    error_code=NULL, error_summary=NULL, updated_at=now()
		WHERE id=$1`, payload.DecisionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE interview_sessions SET phase='deciding', updated_at=now() WHERE id=$1 AND user_id=$2`, payload.SessionID, payload.UserID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE async_tasks SET status='running', started_at=COALESCE(started_at,now()), updated_at=now()
		WHERE id=$1 AND user_id=$2`, payload.TaskID, payload.UserID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func loadNextTurnRuntime(ctx context.Context, db *pgxpool.Pool, payload sharedtasks.InterviewNextTurnPayload) (nextTurnRuntime, error) {
	var out nextTurnRuntime
	var blueprintRaw, progressRaw []byte
	var skippedAt *time.Time
	err := db.QueryRow(ctx, `
		SELECT session.id::text, session.user_id::text, session.resume_version_id::text,
		       session.question_set_id::text, session.primary_language, session.target_company,
		       COALESCE(qset.target_role,'backend_development'), session.agent_mode,
		       session.current_ordinal, session.follow_ups_used, COALESCE(session.started_at,now()),
		       session.blueprint, session.capability_progress,
		       turn.id::text, turn.turn_kind, COALESCE(turn.capability_key,''), turn.question,
		       COALESCE(turn.answer,''), turn.skipped_at,
		       (SELECT count(*)::int FROM interview_turns done WHERE done.session_id=session.id AND (done.answer IS NOT NULL OR done.skipped_at IS NOT NULL))
		FROM interview_sessions AS session
		JOIN question_sets AS qset ON qset.id=session.question_set_id
		JOIN interview_turns AS turn ON turn.session_id=session.id AND turn.id=$4
		JOIN interview_turn_decisions AS decision ON decision.id=$3 AND decision.answered_turn_id=turn.id
		WHERE session.id=$1 AND session.user_id=$2 AND session.status='active' AND session.phase='deciding'
		  AND session.current_ordinal=turn.ordinal AND decision.status='running'`,
		payload.SessionID, payload.UserID, payload.DecisionID, payload.AnsweredTurnID,
	).Scan(
		&out.SessionID, &out.UserID, &out.ResumeVersionID, &out.QuestionSetID,
		&out.Language, &out.Company, &out.TargetRole, &out.AgentMode,
		&out.CurrentOrdinal, &out.FollowUpsUsed, &out.StartedAt, &blueprintRaw, &progressRaw,
		&out.AnsweredTurnID, &out.TurnKind, &out.Capability, &out.Question, &out.Answer, &skippedAt, &out.CompletedTurns,
	)
	if err != nil {
		return out, err
	}
	out.Skipped = skippedAt != nil
	if err := json.Unmarshal(blueprintRaw, &out.Blueprint); err != nil {
		return out, err
	}
	out.Progress = map[string]contract.CapabilityProgress{}
	if err := json.Unmarshal(progressRaw, &out.Progress); err != nil {
		return out, err
	}
	out.FollowUpDepth = loadWorkerFollowUpDepth(ctx, db, out.AnsweredTurnID)
	out.AllowedFactIDs = loadWorkerFactIDs(ctx, db, out.SessionID, out.UserID)
	out.RecentTurns = loadWorkerRecentTurns(ctx, db, out.SessionID, out.CurrentOrdinal)
	return out, nil
}

func appendNextTurn(ctx context.Context, db *pgxpool.Pool, payload sharedtasks.InterviewNextTurnPayload, runtime nextTurnRuntime, decision contract.NextTurnDecisionV2, response platformai.GenerateResponse, progress map[string]contract.CapabilityProgress, generationMode string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockRunnableDecision(ctx, tx, payload, runtime.CurrentOrdinal); err != nil {
		return err
	}
	nextTurnID := uuid.NewString()
	nextOrdinal := runtime.CurrentOrdinal + 1
	var parent any
	if decision.TurnKind == contract.TurnKindFollowUp {
		parent = runtime.AnsweredTurnID
	}
	progressRaw, _ := json.Marshal(progress)
	coverageRaw, _ := json.Marshal(decision.CoverageObservation)
	if _, err := tx.Exec(ctx, `
		INSERT INTO interview_turns (
			id, session_id, ordinal, question, started_at, turn_kind, parent_turn_id,
			capability_key, intent, expected_points, difficulty, evidence_fact_ids,
			decision_reason, coverage_observation, generation_mode, invocation_id
		) VALUES ($1,$2,$3,$4,now(),$5,NULLIF($6,'')::uuid,$7,$8,$9,$10,$11,$12,$13,$14,NULLIF($15,'')::uuid)`,
		nextTurnID, runtime.SessionID, nextOrdinal, decision.Question, decision.TurnKind, stringValue(parent),
		decision.CapabilityKey, decision.Intent, mustJSON(decision.ExpectedPoints), decision.Difficulty,
		mustJSON(decision.EvidenceFactIDs), decision.Reason, coverageRaw, generationMode, response.InvocationID,
	); err != nil {
		return err
	}
	followIncrement := 0
	if decision.TurnKind == contract.TurnKindFollowUp {
		followIncrement = 1
	}
	if _, err := tx.Exec(ctx, `
		UPDATE interview_sessions
		SET phase='answering', current_ordinal=$2, current_capability_key=$3,
		    follow_ups_used=follow_ups_used+$4, capability_progress=$5,
		    decision_version=decision_version+1,
		    interviewer_model=COALESCE(NULLIF($6,''),interviewer_model), updated_at=now()
		WHERE id=$1 AND user_id=$7`, runtime.SessionID, nextOrdinal, decision.CapabilityKey,
		followIncrement, progressRaw, response.Model, runtime.UserID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE interview_turn_decisions
		SET status='succeeded', action=$2, next_turn_id=$3, model_invocation_id=NULLIF($4,'')::uuid,
		    completed_at=now(), updated_at=now()
		WHERE id=$1 AND status='running'`, payload.DecisionID, decision.Action, nextTurnID, response.InvocationID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE async_tasks SET status='succeeded', progress=100, completed_at=now(),
		    result=$2, updated_at=now() WHERE id=$1 AND user_id=$3 AND status='running'`,
		payload.TaskID, mustJSON(map[string]any{"session_id": runtime.SessionID, "next_ordinal": nextOrdinal}), runtime.UserID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func completeNextTurn(ctx context.Context, db *pgxpool.Pool, payload sharedtasks.InterviewNextTurnPayload, runtime nextTurnRuntime, decision contract.NextTurnDecisionV2, response platformai.GenerateResponse, progress map[string]contract.CapabilityProgress, reason string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockRunnableDecision(ctx, tx, payload, runtime.CurrentOrdinal); err != nil {
		return err
	}
	progressRaw, _ := json.Marshal(progress)
	if _, err := tx.Exec(ctx, `
		UPDATE interview_sessions
		SET status='completed', phase='completed', completed_at=now(),
		    duration_seconds=GREATEST(0,FLOOR(EXTRACT(EPOCH FROM (now()-started_at)))::int),
		    capability_progress=$2, completion_reason=$3, decision_version=decision_version+1, updated_at=now()
		WHERE id=$1 AND user_id=$4`, runtime.SessionID, progressRaw, reason, runtime.UserID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE interview_turn_decisions
		SET status='succeeded', action=$2, model_invocation_id=NULLIF($3,'')::uuid,
		    completed_at=now(), updated_at=now()
		WHERE id=$1 AND status='running'`, payload.DecisionID, decision.Action, response.InvocationID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE async_tasks SET status='succeeded', progress=100, completed_at=now(), result=$2, updated_at=now()
		WHERE id=$1 AND user_id=$3 AND status='running'`,
		payload.TaskID, mustJSON(map[string]any{"session_id": runtime.SessionID, "completed": true}), runtime.UserID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func lockRunnableDecision(ctx context.Context, tx pgx.Tx, payload sharedtasks.InterviewNextTurnPayload, ordinal int) error {
	var id string
	return tx.QueryRow(ctx, `
		SELECT decision.id::text
		FROM interview_turn_decisions AS decision
		JOIN interview_sessions AS session ON session.id=decision.session_id
		WHERE decision.id=$1 AND decision.user_id=$2 AND decision.status='running'
		  AND session.id=$3 AND session.user_id=$2 AND session.status='active'
		  AND session.phase='deciding' AND session.current_ordinal=$4
		FOR UPDATE OF decision, session`, payload.DecisionID, payload.UserID, payload.SessionID, ordinal,
	).Scan(&id)
}

func failNextTurn(ctx context.Context, db *pgxpool.Pool, payload sharedtasks.InterviewNextTurnPayload, code, summary string, cause error) error {
	tx, err := db.Begin(ctx)
	if err == nil {
		defer tx.Rollback(ctx)
		_, _ = tx.Exec(ctx, `
			UPDATE interview_turn_decisions SET status='failed', error_code=$2, error_summary=$3,
			    completed_at=now(), updated_at=now()
			WHERE id=$1 AND user_id=$4 AND status='running'`, payload.DecisionID, code, summary, payload.UserID)
		_, _ = tx.Exec(ctx, `
			UPDATE interview_sessions SET phase='decision_failed', updated_at=now()
			WHERE id=$1 AND user_id=$2 AND status='active' AND phase='deciding'`, payload.SessionID, payload.UserID)
		_, _ = tx.Exec(ctx, `
			UPDATE async_tasks SET status='failed', error_code=$2, error_summary=$3, error_message=$4,
			    completed_at=now(), updated_at=now()
			WHERE id=$1 AND user_id=$5 AND status='running'`, payload.TaskID, code, summary, cause.Error(), payload.UserID)
		_ = tx.Commit(ctx)
	}
	return fmt.Errorf("%w: %v", asynq.SkipRetry, cause)
}

func fallbackNextTurnDecision(ctx context.Context, db *pgxpool.Pool, runtime nextTurnRuntime, reason string) (contract.NextTurnDecisionV2, error) {
	preferred := nextCapability(runtime.Blueprint, runtime.Progress, runtime.Capability)
	var question, intent, capability, difficulty string
	var expectedRaw, evidenceRaw []byte
	err := db.QueryRow(ctx, `
		SELECT material.question, material.intent, COALESCE(material.capability_key,''),
		       COALESCE(material.difficulty,'medium'), material.expected_points, material.evidence_fact_ids
		FROM questions AS material
		WHERE material.question_set_id=$1
		  AND material.material_kind IN ('anchor','fallback')
		  AND NOT EXISTS (
			SELECT 1 FROM interview_turns used
			WHERE used.session_id=$2 AND used.question=material.question
		  )
		ORDER BY CASE WHEN material.capability_key=$3 THEN 0 ELSE 1 END,
		         CASE material.material_kind WHEN 'anchor' THEN 0 ELSE 1 END,
		         material.ordinal
		LIMIT 1`, runtime.QuestionSetID, runtime.SessionID, preferred,
	).Scan(&question, &intent, &capability, &difficulty, &expectedRaw, &evidenceRaw)
	if err != nil {
		return contract.NextTurnDecisionV2{}, err
	}
	var expected, evidence []string
	_ = json.Unmarshal(expectedRaw, &expected)
	_ = json.Unmarshal(evidenceRaw, &evidence)
	return contract.NextTurnDecisionV2{
		Action: contract.ActionSwitchCapability, Question: question, TurnKind: contract.TurnKindMain,
		CapabilityKey: capability, Intent: intent, ExpectedPoints: expected, Difficulty: difficulty,
		EvidenceFactIDs: evidence, Reason: reason,
		CoverageObservation: contract.CoverageObservation{EvidenceQuality: fallbackEvidenceQuality(runtime.Answer), Resolved: []string{}, Unresolved: []string{"需要后续问题补充验证"}},
	}, nil
}

func updateCapabilityProgress(progress map[string]contract.CapabilityProgress, capability string, observation contract.CoverageObservation, turnKind string) map[string]contract.CapabilityProgress {
	result := make(map[string]contract.CapabilityProgress, len(progress))
	for key, value := range progress {
		result[key] = value
	}
	item := result[capability]
	item.AskedTurns++
	if turnKind == contract.TurnKindFollowUp {
		item.FollowUpTurns++
	}
	if observation.EvidenceQuality > 0 {
		item.EvidenceCount++
	}
	if item.AskedTurns == 1 {
		item.EvidenceQuality = observation.EvidenceQuality
	} else {
		item.EvidenceQuality = (item.EvidenceQuality*(item.AskedTurns-1) + observation.EvidenceQuality) / item.AskedTurns
	}
	item.CoverageScore = item.EvidenceQuality
	item.UnresolvedGaps = append([]string(nil), observation.Unresolved...)
	result[capability] = item
	return result
}

func coveredWeight(blueprint contract.InterviewBlueprintV2, progress map[string]contract.CapabilityProgress) int {
	weight := 0
	for _, capability := range blueprint.Capabilities {
		item := progress[capability.Key]
		if item.EvidenceCount >= capability.TargetEvidence || item.CoverageScore >= 70 {
			weight += capability.Weight
		}
	}
	return weight
}

func criticalCapabilitiesCovered(blueprint contract.InterviewBlueprintV2, progress map[string]contract.CapabilityProgress) bool {
	for _, capability := range blueprint.Capabilities {
		if capability.Weight >= 25 {
			item := progress[capability.Key]
			if item.EvidenceCount < capability.TargetEvidence && item.CoverageScore < 60 {
				return false
			}
		}
	}
	return true
}

func nextCapability(blueprint contract.InterviewBlueprintV2, progress map[string]contract.CapabilityProgress, current string) string {
	best := ""
	bestScore := int(^uint(0) >> 1)
	for _, capability := range blueprint.Capabilities {
		item := progress[capability.Key]
		score := item.AskedTurns*100 - capability.Weight
		if capability.Key == current {
			score += 20
		}
		if score < bestScore {
			best, bestScore = capability.Key, score
		}
	}
	return best
}

func loadWorkerFollowUpDepth(ctx context.Context, db *pgxpool.Pool, turnID string) int {
	var depth int
	_ = db.QueryRow(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT id,parent_turn_id,0 AS depth FROM interview_turns WHERE id=$1
			UNION ALL
			SELECT parent.id,parent.parent_turn_id,ancestors.depth+1
			FROM interview_turns parent JOIN ancestors ON parent.id=ancestors.parent_turn_id
		)
		SELECT COALESCE(max(depth),0) FROM ancestors`, turnID).Scan(&depth)
	return depth
}

func loadWorkerFactIDs(ctx context.Context, db *pgxpool.Pool, sessionID, userID string) []string {
	rows, err := db.Query(ctx, `
		SELECT fact.id::text FROM resume_facts fact
		JOIN interview_sessions session ON session.resume_version_id=fact.resume_version_id
		WHERE session.id=$1 AND session.user_id=$2 ORDER BY fact.created_at`, sessionID, userID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			result = append(result, id)
		}
	}
	return result
}

func loadWorkerRecentTurns(ctx context.Context, db *pgxpool.Pool, sessionID string, ordinal int) []map[string]any {
	rows, err := db.Query(ctx, `
		SELECT ordinal,turn_kind,question,COALESCE(answer,''),COALESCE(capability_key,'')
		FROM interview_turns WHERE session_id=$1 AND ordinal<=$2 ORDER BY ordinal DESC LIMIT 6`, sessionID, ordinal)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var ordinal int
		var kind, question, answer, capability string
		if rows.Scan(&ordinal, &kind, &question, &answer, &capability) == nil {
			result = append(result, map[string]any{"ordinal": ordinal, "turn_kind": kind, "question": platformai.ClipRunes(question, 500), "answer": platformai.ClipRunes(answer, 1000), "capability_key": capability})
		}
	}
	return result
}

func fallbackEvidenceQuality(answer string) int {
	length := len([]rune(strings.TrimSpace(answer)))
	if length >= 240 {
		return 70
	}
	if length >= 80 {
		return 55
	}
	if length > 0 {
		return 35
	}
	return 0
}

func elapsedMinutes(start time.Time) int {
	if start.IsZero() {
		return 0
	}
	minutes := int(time.Since(start).Minutes())
	if minutes < 0 {
		return 0
	}
	return minutes
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if result, ok := value.(string); ok {
		return result
	}
	return fmt.Sprint(value)
}

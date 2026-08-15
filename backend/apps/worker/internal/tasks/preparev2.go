package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/internal/aiworkflow"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type preparedMaterial struct {
	ID              string
	Ordinal         int
	Question        string
	Intent          string
	ExpectedPoints  []string
	EvidenceFactIDs []string
	CapabilityKey   string
	Difficulty      string
	Kind            string
}

// InterviewPrepareHandler creates an executable blueprint, hidden safety
// materials, and only the first visible turn. It never pre-creates a queue of
// public interview questions.
func InterviewPrepareHandler(db *pgxpool.Pool, chat platformai.ChatModel) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload sharedtasks.QuestionGeneratePayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("%w: decode interview prepare task: %v", asynq.SkipRetry, err)
		}
		var existingStatus string
		err := db.QueryRow(ctx, `
			UPDATE async_tasks AS task
			SET status='running', started_at=COALESCE(started_at, now()), updated_at=now()
			WHERE task.id=$1 AND task.user_id=$2 AND task.ref_id=$3
			  AND task.task_type='interview.prepare'
			  AND (task.status='pending' OR (task.status='running' AND task.updated_at < now() - interval '10 minutes'))
			  AND EXISTS (
				SELECT 1
				FROM interview_sessions AS session
				JOIN question_sets AS qset ON qset.id=session.question_set_id
				WHERE session.id=$3 AND session.user_id=$2
				  AND session.resume_id=$4 AND session.resume_version_id=$6
				  AND qset.id=$5 AND qset.resume_version_id=$6
				  AND session.status='preparing' AND session.phase='preparing'
				  AND qset.status='generating'
			  )
			RETURNING task.status::text`,
			payload.TaskID, payload.UserID, payload.SessionID, payload.ResumeID, payload.QuestionSetID, payload.ResumeVersionID,
		).Scan(&existingStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			_ = db.QueryRow(ctx, `SELECT status::text FROM async_tasks WHERE id=$1 AND user_id=$2`, payload.TaskID, payload.UserID).Scan(&existingStatus)
			if existingStatus == "succeeded" {
				return nil
			}
			return fmt.Errorf("%w: preparation task is stale or no longer runnable", asynq.SkipRetry)
		}
		if err != nil {
			return err
		}
		updatePrepareProgress(ctx, db, payload, 10, "reading_materials")

		var resumeText string
		if err := db.QueryRow(ctx, `
			SELECT COALESCE(version.extracted_text, ''),session.primary_language,
			       session.target_company,COALESCE(qset.target_role,'backend_development')
			FROM resumes AS resume
			JOIN resume_versions AS version ON version.resume_id=resume.id
			JOIN interview_sessions AS session ON session.resume_id=resume.id AND session.resume_version_id=version.id
			JOIN question_sets AS qset ON qset.id=session.question_set_id
			WHERE resume.id=$1 AND resume.user_id=$2 AND version.id=$3
			  AND session.id=$4 AND session.user_id=$2 AND qset.id=$5`,
			payload.ResumeID, payload.UserID, payload.ResumeVersionID, payload.SessionID, payload.QuestionSetID,
		).Scan(&resumeText, &payload.PrimaryLanguage, &payload.TargetCompany, &payload.TargetRole); err != nil {
			return failInterviewPrepare(ctx, db, payload, "INTERVIEW_MATERIALS_UNAVAILABLE", "无法读取面试材料", err)
		}
		facts, factIDs, err := loadVersionFacts(ctx, db, payload)
		if err != nil {
			return failInterviewPrepare(ctx, db, payload, "INTERVIEW_MATERIALS_UNAVAILABLE", "无法读取简历事实", err)
		}
		interviewContext := fmt.Sprintf("固定岗位：后端开发\n主要语言：%s\n目标公司：%s", payload.PrimaryLanguage, payload.TargetCompany)

		mode := "rule"
		turnGenerationMode := "fallback"
		modelName := "rule"
		setStatus := "degraded"
		blueprint := ruleBlueprintV2()
		materials := ruleMaterialsV2(blueprint, payload.PrimaryLanguage)
		var firstInvocationID string
		if chat != nil {
			mode = "ai"
			turnGenerationMode = "ai"
			setStatus = "ready"
			updatePrepareProgress(ctx, db, payload, 30, "planning_capabilities")
			generatedBlueprint, blueprintResponse, generateErr := aiworkflow.GenerateBlueprintV2(
				ctx, chat, payload.UserID, payload.TaskID, payload.QuestionSetID,
				resumeText, interviewContext, payload.TargetRole, facts,
			)
			if generateErr != nil {
				return failInterviewPrepare(ctx, db, payload, aiFailureCode(generateErr), "面试能力规划生成失败，请重试", generateErr)
			}
			blueprint = generatedBlueprint
			modelName = blueprintResponse.Model
			updatePrepareProgress(ctx, db, payload, 60, "generating_materials")
			generatedMaterials, materialsResponse, generateErr := aiworkflow.GenerateInterviewMaterials(
				ctx, chat, payload.UserID, payload.TaskID, payload.QuestionSetID,
				resumeText, interviewContext, payload.TargetRole, blueprint, factIDs,
			)
			if generateErr != nil {
				return failInterviewPrepare(ctx, db, payload, aiFailureCode(generateErr), "面试材料生成失败，请重试", generateErr)
			}
			materials = generatedMaterials
			if strings.TrimSpace(materialsResponse.Model) != "" {
				modelName = materialsResponse.Model
			}
			firstInvocationID = materialsResponse.InvocationID
		}
		items, err := flattenMaterials(blueprint, materials)
		if err != nil {
			return failInterviewPrepare(ctx, db, payload, "AI_OUTPUT_INVALID", "面试材料不完整，请重试", err)
		}
		updatePrepareProgress(ctx, db, payload, 80, "quality_check")
		blueprintJSON, _ := json.Marshal(blueprint)
		progressJSON := initialCapabilityProgress(blueprint)
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		failWrite := func(code, summary string, cause error) error {
			_ = tx.Rollback(ctx)
			return failInterviewPrepare(ctx, db, payload, code, summary, cause)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM questions WHERE question_set_id=$1`, payload.QuestionSetID); err != nil {
			return failWrite("INTERVIEW_PREPARE_WRITE_FAILED", "保存面试材料失败", err)
		}
		for _, item := range items {
			if _, err := tx.Exec(ctx, `
				INSERT INTO questions (
					id, question_set_id, ordinal, question, intent, expected_points,
					evidence_fact_ids, capability_key, difficulty, material_kind
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				item.ID, payload.QuestionSetID, item.Ordinal, item.Question, item.Intent,
				mustJSON(item.ExpectedPoints), mustJSON(item.EvidenceFactIDs), item.CapabilityKey, item.Difficulty, item.Kind,
			); err != nil {
				return failWrite("INTERVIEW_PREPARE_WRITE_FAILED", "保存面试材料失败", err)
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE question_sets
			SET status=$2::question_set_status, blueprint=$3, prompt_version=$4, model_name=$5, updated_at=now()
			WHERE id=$1 AND user_id=$6`,
			payload.QuestionSetID, setStatus, blueprintJSON, aiworkflow.BlueprintV2Version, modelName, payload.UserID,
		); err != nil {
			return failWrite("INTERVIEW_PREPARE_WRITE_FAILED", "保存面试规划失败", err)
		}
		first := items[0]
		if _, err := tx.Exec(ctx, `
			INSERT INTO interview_turns (
				id, session_id, ordinal, question, started_at, turn_kind, capability_key,
				source_question_id, intent, expected_points, difficulty, evidence_fact_ids,
				generation_mode, invocation_id
			) VALUES ($1,$2,1,$3,now(),'main',$4,$5,$6,$7,$8,$9,$10,NULLIF($11,'')::uuid)
			ON CONFLICT (session_id, ordinal) DO NOTHING`,
			uuid.NewString(), payload.SessionID, first.Question, first.CapabilityKey, first.ID,
			first.Intent, mustJSON(first.ExpectedPoints), first.Difficulty, mustJSON(first.EvidenceFactIDs), turnGenerationMode, firstInvocationID,
		); err != nil {
			return failWrite("INTERVIEW_PREPARE_WRITE_FAILED", "初始化首题失败", err)
		}
		activation, err := tx.Exec(ctx, `
			UPDATE interview_sessions
			SET status='active', phase='answering', current_ordinal=1,
			    started_at=COALESCE(started_at, now()), blueprint=$2,
			    agent_mode=$3, policy_version='standard-v2', interviewer_prompt_version=$4,
			    min_turns=$5, target_turns=$6, max_turns=$7, time_budget_minutes=$8,
			    max_follow_up_depth=$9, max_follow_ups_total=$10,
			    follow_up_budget=$10, current_capability_key=$11,
			    interviewer_model=$12, capability_progress=$13, updated_at=now()
			WHERE id=$1 AND user_id=$14 AND question_set_id=$15
			  AND status='preparing' AND phase='preparing'`,
			payload.SessionID, blueprintJSON, mode, aiworkflow.NextTurnV2Version,
			blueprint.MinTurns, blueprint.TargetTurns, blueprint.MaxTurns, blueprint.TimeBudgetMinutes,
			blueprint.MaxFollowUpDepth, blueprint.MaxFollowUpsTotal, first.CapabilityKey,
			modelName, progressJSON, payload.UserID, payload.QuestionSetID,
		)
		if err != nil {
			return failWrite("INTERVIEW_PREPARE_WRITE_FAILED", "激活面试失败", err)
		}
		if activation.RowsAffected() != 1 {
			return failWrite("INTERVIEW_PREPARE_STALE", "面试准备任务已过期", fmt.Errorf("activation affected %d rows", activation.RowsAffected()))
		}
		if _, err := tx.Exec(ctx, `
			UPDATE async_tasks
			SET status='succeeded', progress=100, result=$2, completed_at=now(), updated_at=now()
			WHERE id=$1 AND user_id=$3 AND status='running'`,
			payload.TaskID, mustJSON(map[string]string{"stage": "completed", "session_id": payload.SessionID}), payload.UserID,
		); err != nil {
			return failWrite("INTERVIEW_PREPARE_WRITE_FAILED", "保存准备任务状态失败", err)
		}
		return tx.Commit(ctx)
	}
}

func loadVersionFacts(ctx context.Context, db *pgxpool.Pool, payload sharedtasks.QuestionGeneratePayload) ([]contract.ResumeFact, map[string]struct{}, error) {
	rows, err := db.Query(ctx, `
		SELECT fact.id::text, fact.fact_type, fact.fact_key, fact.fact_value, fact.source_excerpt, fact.confidence::float8
		FROM resume_facts AS fact
		JOIN resume_versions AS version ON version.id=fact.resume_version_id
		JOIN resumes AS resume ON resume.id=version.resume_id
		WHERE resume.id=$1 AND resume.user_id=$2 AND version.id=$3
		ORDER BY fact.created_at ASC`, payload.ResumeID, payload.UserID, payload.ResumeVersionID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	facts := []contract.ResumeFact{}
	ids := map[string]struct{}{}
	for rows.Next() {
		var id string
		var fact contract.ResumeFact
		var raw []byte
		if err := rows.Scan(&id, &fact.FactType, &fact.FactKey, &raw, &fact.SourceExcerpt, &fact.Confidence); err != nil {
			return nil, nil, err
		}
		_ = json.Unmarshal(raw, &fact.FactValue)
		facts = append(facts, fact)
		ids[id] = struct{}{}
	}
	return facts, ids, rows.Err()
}

func flattenMaterials(blueprint contract.InterviewBlueprintV2, materials contract.InterviewMaterials) ([]preparedMaterial, error) {
	plans := map[string]contract.CapabilityPlan{}
	for _, plan := range blueprint.Capabilities {
		plans[plan.Key] = plan
	}
	items := []preparedMaterial{}
	ordinal := 1
	for _, material := range materials.Capabilities {
		plan, ok := plans[material.CapabilityKey]
		if !ok {
			return nil, fmt.Errorf("unknown capability %s", material.CapabilityKey)
		}
		difficulty := "medium"
		if len(plan.DifficultyCurve) > 0 {
			difficulty = plan.DifficultyCurve[0]
		}
		for _, question := range material.AnchorQuestions {
			items = append(items, preparedMaterial{ID: uuid.NewString(), Ordinal: ordinal, Question: question, Intent: plan.Label, ExpectedPoints: material.ExpectedEvidence, EvidenceFactIDs: material.EvidenceFactIDs, CapabilityKey: plan.Key, Difficulty: difficulty, Kind: "anchor"})
			ordinal++
		}
		for _, question := range material.FallbackQuestions {
			items = append(items, preparedMaterial{ID: uuid.NewString(), Ordinal: ordinal, Question: question, Intent: plan.Label, ExpectedPoints: material.ExpectedEvidence, EvidenceFactIDs: material.EvidenceFactIDs, CapabilityKey: plan.Key, Difficulty: difficulty, Kind: "fallback"})
			ordinal++
		}
	}
	if len(items) == 0 {
		return nil, errors.New("no interview materials")
	}
	return items, nil
}

func ruleBlueprintV2() contract.InterviewBlueprintV2 {
	value := contract.DefaultInterviewBlueprintV2()
	value.Capabilities = []contract.CapabilityPlan{
		{Key: "language", Label: "语言与工程基础", Weight: 25, TargetEvidence: 2, DifficultyCurve: []string{"easy", "medium", "hard"}, Rubric: []string{"语言机制理解", "工程实践与取舍"}},
		{Key: "project", Label: "项目深挖", Weight: 30, TargetEvidence: 2, DifficultyCurve: []string{"medium", "hard"}, Rubric: []string{"个人贡献可验证", "结果与复盘完整"}},
		{Key: "systems", Label: "系统设计", Weight: 30, TargetEvidence: 2, DifficultyCurve: []string{"medium", "hard"}, Rubric: []string{"约束识别", "方案权衡与演进"}},
		{Key: "incident", Label: "故障处理", Weight: 15, TargetEvidence: 1, DifficultyCurve: []string{"medium", "hard"}, Rubric: []string{"定位与止损", "根因和防再发"}},
	}
	value.PromptVersion = aiworkflow.BlueprintV2Version
	value.Model = "rule"
	return value
}

func ruleMaterialsV2(blueprint contract.InterviewBlueprintV2, language string) contract.InterviewMaterials {
	questions := map[string][2]string{
		"language": {fmt.Sprintf("请结合实际项目说明你使用 %s 时最重要的一项工程取舍。", language), fmt.Sprintf("%s 服务中你如何处理并发、错误和资源释放？", language)},
		"project":  {"选择简历中最有挑战的后端项目，说明你的个人贡献和可验证结果。", "如果重新实施这个项目，你会优先改变哪项技术决策？"},
		"systems":  {"设计一个高并发订单接口，说明容量、存储、一致性和故障策略。", "如果流量增长十倍，你会先验证并改造哪个瓶颈？"},
		"incident": {"讲述一次线上故障：你如何发现、止损、定位根因并防止复发？", "如果监控没有直接暴露根因，你会怎样逐步缩小范围？"},
	}
	result := contract.InterviewMaterials{SchemaVersion: "v1", Capabilities: []contract.CapabilityMaterial{}}
	for _, plan := range blueprint.Capabilities {
		pair := questions[plan.Key]
		result.Capabilities = append(result.Capabilities, contract.CapabilityMaterial{
			CapabilityKey: plan.Key, ExpectedEvidence: append([]string(nil), plan.Rubric...),
			AnchorQuestions:   []string{pair[0], fmt.Sprintf("围绕%s，请说明一个失败案例以及你从中修正的判断。", plan.Label)},
			FallbackQuestions: []string{pair[1], fmt.Sprintf("在%s场景中遇到信息不足时，你如何验证假设并作出取舍？", plan.Label)},
			EvidenceFactIDs:   []string{},
		})
	}
	return result
}

func initialCapabilityProgress(blueprint contract.InterviewBlueprintV2) []byte {
	progress := make(map[string]contract.CapabilityProgress, len(blueprint.Capabilities))
	for _, capability := range blueprint.Capabilities {
		progress[capability.Key] = contract.CapabilityProgress{}
	}
	data, _ := json.Marshal(progress)
	return data
}

func updatePrepareProgress(ctx context.Context, db *pgxpool.Pool, payload sharedtasks.QuestionGeneratePayload, progress int, stage string) {
	_, _ = db.Exec(ctx, `
		UPDATE async_tasks
		SET status='running', progress=$2, result=$3, updated_at=now()
		WHERE id=$1 AND user_id=$4 AND status='running'`,
		payload.TaskID, progress, mustJSON(map[string]string{"stage": stage, "session_id": payload.SessionID}), payload.UserID)
}

func failInterviewPrepare(ctx context.Context, db *pgxpool.Pool, payload sharedtasks.QuestionGeneratePayload, code, summary string, cause error) error {
	_, _ = db.Exec(ctx, `UPDATE question_sets SET status='failed', updated_at=now() WHERE id=$1 AND user_id=$2 AND status='generating'`, payload.QuestionSetID, payload.UserID)
	_, _ = db.Exec(ctx, `
		UPDATE interview_sessions SET status='failed', phase='preparing', updated_at=now()
		WHERE id=$1 AND user_id=$2 AND question_set_id=$3 AND status='preparing'`, payload.SessionID, payload.UserID, payload.QuestionSetID)
	_, _ = db.Exec(ctx, `
		UPDATE async_tasks
		SET status='failed', error_code=$2, error_summary=$3, error_message=$4, completed_at=now(), updated_at=now()
		WHERE id=$1 AND user_id=$5 AND status='running'`, payload.TaskID, code, summary, cause.Error(), payload.UserID)
	return fmt.Errorf("%w: %v", asynq.SkipRetry, cause)
}

func aiFailureCode(err error) string {
	if platformai.IsErrorCode(err, platformai.ErrorOutputInvalid) {
		return "AI_OUTPUT_INVALID"
	}
	if platformai.IsErrorCode(err, platformai.ErrorBudgetExhausted) {
		return "AI_BUDGET_EXHAUSTED"
	}
	return "AI_GENERATION_UNAVAILABLE"
}

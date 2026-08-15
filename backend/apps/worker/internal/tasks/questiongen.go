package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/internal/aiworkflow"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func QuestionGenerateHandler(db *pgxpool.Pool, chat platformai.ChatModel) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload sharedtasks.QuestionGeneratePayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("%w: decode task: %v", asynq.SkipRetry, err)
		}
		var claimedStatus string
		err := db.QueryRow(ctx, `
			UPDATE async_tasks AS task
			SET status='running', started_at=COALESCE(started_at, now()), updated_at=now()
			WHERE task.id=$1 AND task.user_id=$2 AND task.ref_id=$3
			  AND task.task_type='question.generate'
			  AND (
			    task.status='pending'
			    OR (task.status='running' AND task.updated_at < now() - interval '10 minutes')
			  )
			  AND EXISTS (
				SELECT 1
				FROM interview_sessions AS session
				JOIN question_sets AS qset ON qset.id=session.question_set_id
				WHERE session.id=$3 AND session.user_id=$2
				  AND session.resume_id=$4 AND session.resume_version_id=$6
				  AND qset.id=$5 AND qset.resume_version_id=$6
				  AND session.status='preparing' AND qset.status='generating'
			  )
			RETURNING task.status::text`,
			payload.TaskID, payload.UserID, payload.SessionID, payload.ResumeID, payload.QuestionSetID, payload.ResumeVersionID,
		).Scan(&claimedStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			var existingStatus string
			_ = db.QueryRow(ctx, `
				SELECT status::text FROM async_tasks
				WHERE id=$1 AND user_id=$2 AND task_type='question.generate'`,
				payload.TaskID, payload.UserID,
			).Scan(&existingStatus)
			if existingStatus == "succeeded" {
				return nil
			}
			if existingStatus == "running" {
				return fmt.Errorf("preparation task is already running")
			}
			return fmt.Errorf("%w: preparation task is stale or no longer runnable", asynq.SkipRetry)
		}
		if err != nil {
			return err
		}
		updateTask := func(progress int, stage string) {
			result, _ := json.Marshal(map[string]string{"stage": stage, "session_id": payload.SessionID})
			_, _ = db.Exec(ctx, `
				UPDATE async_tasks
				SET status='running', progress=$2, result=$3,
				    started_at=COALESCE(started_at, now()), updated_at=now()
				WHERE id=$1 AND user_id=$4 AND status='running'`, payload.TaskID, progress, result, payload.UserID)
		}
		updateTask(10, "reading_materials")

		var resumeText string
		err = db.QueryRow(ctx, `
			SELECT COALESCE(version.extracted_text, '')
			FROM resumes AS resume
			JOIN resume_versions AS version ON version.resume_id = resume.id
			WHERE resume.id=$1 AND resume.user_id=$2 AND version.id=$3`,
			payload.ResumeID, payload.UserID, payload.ResumeVersionID,
		).Scan(&resumeText)
		if err != nil {
			return failQuestionSet(ctx, db, payload, "QUESTION_MATERIALS_UNAVAILABLE", "无法读取简历材料", err)
		}
		jobText := fmt.Sprintf(
			"固定岗位：后端开发（backend_development）\n主要技术语言：%s\n目标公司：%s\n请围绕该语言的后端基础、项目深挖、系统设计和目标公司特点设计面试。",
			payload.PrimaryLanguage,
			payload.TargetCompany,
		)
		factIDs := map[string]struct{}{}
		facts := []contract.ResumeFact{}
		rows, err := db.Query(ctx, `
			SELECT fact.id::text, fact.fact_type, fact.fact_key, fact.fact_value, fact.source_excerpt, fact.confidence::float8
			FROM resume_facts AS fact
			JOIN resume_versions AS version ON version.id = fact.resume_version_id
			JOIN resumes AS resume ON resume.id = version.resume_id
			WHERE resume.id=$1 AND resume.user_id=$2 AND version.id=$3`,
			payload.ResumeID, payload.UserID, payload.ResumeVersionID)
		if err != nil {
			return failQuestionSet(ctx, db, payload, "QUESTION_MATERIALS_UNAVAILABLE", "无法读取简历事实", err)
		}
		for rows.Next() {
			var fact contract.ResumeFact
			var id string
			var raw []byte
			if err := rows.Scan(&id, &fact.FactType, &fact.FactKey, &raw, &fact.SourceExcerpt, &fact.Confidence); err != nil {
				rows.Close()
				return failQuestionSet(ctx, db, payload, "QUESTION_MATERIALS_UNAVAILABLE", "无法读取简历事实", err)
			}
			_ = json.Unmarshal(raw, &fact.FactValue)
			facts = append(facts, fact)
			factIDs[id] = struct{}{}
		}
		rows.Close()

		var questions []contract.GeneratedQuestion
		var blueprint contract.InterviewBlueprint
		modelName := "rule"
		status := "degraded"
		if chat != nil {
			updateTask(30, "planning_capabilities")
			generatedBlueprint, _, err := aiworkflow.GenerateBlueprint(ctx, chat, payload.UserID, payload.TaskID, payload.QuestionSetID, resumeText, jobText, payload.TargetRole, facts)
			if err != nil {
				return failQuestionSet(ctx, db, payload, "AI_GENERATION_UNAVAILABLE", "面试蓝图生成失败，请重试", err)
			}
			blueprint = generatedBlueprint
			updateTask(60, "generating_questions")
			generated, response, err := aiworkflow.GenerateQuestions(ctx, chat, payload.UserID, payload.TaskID, payload.QuestionSetID, resumeText, jobText, payload.TargetRole, blueprint, factIDs)
			if err != nil {
				code := "AI_GENERATION_UNAVAILABLE"
				if platformai.IsErrorCode(err, platformai.ErrorOutputInvalid) {
					code = "AI_OUTPUT_INVALID"
				}
				if platformai.IsErrorCode(err, platformai.ErrorBudgetExhausted) {
					code = "AI_BUDGET_EXHAUSTED"
				}
				return failQuestionSet(ctx, db, payload, code, "面试准备失败，请重试", err)
			}
			questions = generated.Questions
			modelName = response.Model
			status = "ready"
		} else {
			questions = ruleQuestions()
			blueprint = contract.InterviewBlueprint{
				CapabilityKeys:    []string{"intro", "project", "problem", "collaboration", "motivation"},
				Weights:           map[string]int{"intro": 20, "project": 25, "problem": 20, "collaboration": 15, "motivation": 20},
				Difficulty:        "mixed",
				QuestionCount:     5,
				FollowUpBudget:    0,
				TimeBudgetMinutes: 15,
				EvidenceScope:     []string{"resume"},
				SchemaVersion:     "v1",
			}
		}
		updateTask(80, "quality_check")
		blueprintJSON, _ := json.Marshal(blueprint)
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		failWrite := func(code, summary string, cause error) error {
			_ = tx.Rollback(ctx)
			return failQuestionSet(ctx, db, payload, code, summary, cause)
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM questions
			WHERE question_set_id=$1
			  AND EXISTS (
				SELECT 1 FROM question_sets
				WHERE id=$1 AND user_id=$2
			  )`, payload.QuestionSetID, payload.UserID); err != nil {
			return failWrite("QUESTION_WRITE_FAILED", "保存面试准备数据失败", err)
		}
		for _, item := range questions {
			questionID := uuid.NewString()
			if _, err := tx.Exec(ctx, `
				INSERT INTO questions (
					id, question_set_id, ordinal, question, intent, expected_points, follow_up_hint,
					evidence_fact_ids, capability_key, difficulty
				) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,NULLIF($9,''),NULLIF($10,''))`,
				questionID,
				payload.QuestionSetID,
				item.Ordinal,
				item.Question,
				item.Intent,
				mustJSON(item.ExpectedPoints),
				item.FollowUpHint,
				mustJSON(item.EvidenceFactIDs),
				item.CapabilityKey,
				item.Difficulty,
			); err != nil {
				return failWrite("QUESTION_WRITE_FAILED", "保存面试准备数据失败", err)
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE question_sets
			SET status=$2::question_set_status, blueprint=$3, prompt_version=$4, model_name=$5, updated_at=now()
			WHERE id=$1 AND user_id=$6`,
			payload.QuestionSetID, status, blueprintJSON, aiworkflow.PromptV1, modelName, payload.UserID,
		); err != nil {
			return failWrite("QUESTION_WRITE_FAILED", "保存面试准备数据失败", err)
		}
		var firstQuestionID, firstQuestion, firstCapability string
		if err := tx.QueryRow(ctx, `
			SELECT id::text, question, COALESCE(capability_key, '')
			FROM questions
			WHERE question_set_id=$1
			ORDER BY ordinal ASC
			LIMIT 1`, payload.QuestionSetID,
		).Scan(&firstQuestionID, &firstQuestion, &firstCapability); err != nil {
			return failWrite("QUESTION_WRITE_FAILED", "没有可用于开始面试的题目", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO interview_turns (
				id, session_id, ordinal, question, started_at, turn_kind,
				capability_key, source_question_id
			) VALUES ($1,$2,1,$3,now(),'main',NULLIF($4,''),$5)
			ON CONFLICT (session_id, ordinal) DO NOTHING`,
			uuid.NewString(), payload.SessionID, firstQuestion, firstCapability, firstQuestionID,
		); err != nil {
			return failWrite("QUESTION_WRITE_FAILED", "无法初始化面试首题", err)
		}
		activationTag, err := tx.Exec(ctx, `
			UPDATE interview_sessions
			SET status='active', phase='answering', current_ordinal=1, started_at=COALESCE(started_at, now()),
			    blueprint=$2, follow_up_budget=$3, current_capability_key=NULLIF($4,''),
			    interviewer_model=$5, updated_at=now()
			WHERE id=$1 AND user_id=$6 AND question_set_id=$7 AND status='preparing'`,
			payload.SessionID, blueprintJSON, blueprint.FollowUpBudget, firstCapability, modelName,
			payload.UserID, payload.QuestionSetID,
		)
		if err != nil {
			return failWrite("QUESTION_WRITE_FAILED", "无法激活面试", err)
		}
		if activationTag.RowsAffected() != 1 {
			return failWrite("QUESTION_TASK_STALE", "面试准备任务已过期", fmt.Errorf("session activation affected %d rows", activationTag.RowsAffected()))
		}
		if _, err := tx.Exec(ctx, `
			UPDATE async_tasks
			SET status='succeeded', progress=100, result=$2, completed_at=now(), updated_at=now()
			WHERE id=$1 AND user_id=$3 AND status='running'`,
			payload.TaskID, mustJSON(map[string]string{"stage": "completed", "session_id": payload.SessionID}), payload.UserID); err != nil {
			return failWrite("QUESTION_WRITE_FAILED", "保存面试准备数据失败", err)
		}
		return tx.Commit(ctx)
	}
}

func failQuestionSet(ctx context.Context, db *pgxpool.Pool, payload sharedtasks.QuestionGeneratePayload, code, summary string, err error) error {
	_, _ = db.Exec(ctx, `
		UPDATE question_sets SET status='failed'::question_set_status, updated_at=now()
		WHERE id=$1 AND user_id=$2 AND status='generating'`, payload.QuestionSetID, payload.UserID)
	_, _ = db.Exec(ctx, `
		UPDATE interview_sessions SET status='failed', updated_at=now()
		WHERE id=$1 AND user_id=$2 AND question_set_id=$3 AND status='preparing'`,
		payload.SessionID, payload.UserID, payload.QuestionSetID)
	_, _ = db.Exec(ctx, `
		UPDATE async_tasks
		SET status='failed', error_code=$2, error_summary=$3, error_message=$4, completed_at=now(), updated_at=now()
		WHERE id=$1 AND user_id=$5 AND status='running'`, payload.TaskID, code, summary, err.Error(), payload.UserID)
	return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
}

func ruleQuestions() []contract.GeneratedQuestion {
	return []contract.GeneratedQuestion{
		{Ordinal: 1, Question: "请用两分钟介绍你自己，并说明与目标岗位最匹配的经历。", Intent: "自我认知与岗位匹配", ExpectedPoints: []string{"核心经历", "量化结果", "岗位关联"}, FollowUpHint: "其中哪项成果最能代表你的个人贡献？", CapabilityKey: "intro", Difficulty: "easy", Generic: true},
		{Ordinal: 2, Question: "选择简历中最有挑战的项目，说明背景、目标、行动与结果。", Intent: "项目深挖", ExpectedPoints: []string{"STAR 结构", "技术决策", "结果指标"}, FollowUpHint: "如果重新做一次，你会改变什么？", CapabilityKey: "project", Difficulty: "medium", Generic: true},
		{Ordinal: 3, Question: "项目中遇到过什么技术故障或性能瓶颈？你如何定位？", Intent: "问题解决", ExpectedPoints: []string{"定位路径", "证据数据", "复盘改进"}, FollowUpHint: "如何证明修复真正有效？", CapabilityKey: "problem", Difficulty: "medium", Generic: true},
		{Ordinal: 4, Question: "描述一次你与团队成员意见不一致的经历。", Intent: "协作沟通", ExpectedPoints: []string{"分歧背景", "沟通方式", "共同结果"}, FollowUpHint: "你如何处理仍未达成一致的部分？", CapabilityKey: "collaboration", Difficulty: "medium", Generic: true},
		{Ordinal: 5, Question: "为什么申请这个岗位？未来一年希望获得什么成长？", Intent: "动机与规划", ExpectedPoints: []string{"岗位理解", "能力差距", "行动计划"}, FollowUpHint: "你入职前三个月会优先做什么？", CapabilityKey: "motivation", Difficulty: "easy", Generic: true},
	}
}

func mustJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}

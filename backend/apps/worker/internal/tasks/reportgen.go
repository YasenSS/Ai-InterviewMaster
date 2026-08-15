package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/internal/aiworkflow"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

func ReportGenerateHandler(db *pgxpool.Pool, chat platformai.ChatModel) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload sharedtasks.ReportGeneratePayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("%w: decode task: %v", asynq.SkipRetry, err)
		}
		var claimedStatus string
		err := db.QueryRow(ctx, `
			UPDATE async_tasks AS task
			SET status='running', progress=10, started_at=COALESCE(started_at, now()), updated_at=now()
			WHERE task.id=$1 AND task.user_id=$2 AND task.ref_id=$3
			  AND task.task_type='report.generate'
			  AND (
			    task.status='pending'
			    OR (task.status='running' AND task.updated_at < now() - interval '10 minutes')
			  )
			  AND EXISTS (
				SELECT 1 FROM interview_reports AS report
				JOIN interview_sessions AS session ON session.id=report.session_id
				WHERE report.id=$4 AND report.session_id=$3
				  AND session.user_id=$2 AND session.status='completed'
				  AND report.status IN ('pending', 'running')
			  )
			RETURNING task.status::text`,
			payload.TaskID, payload.UserID, payload.SessionID, payload.ReportID,
		).Scan(&claimedStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			var existingStatus string
			_ = db.QueryRow(ctx, `SELECT status::text FROM async_tasks WHERE id=$1 AND user_id=$2`, payload.TaskID, payload.UserID).Scan(&existingStatus)
			if existingStatus == "succeeded" {
				return nil
			}
			if existingStatus == "running" {
				return fmt.Errorf("report task is already running")
			}
			return fmt.Errorf("%w: report task is stale or no longer runnable", asynq.SkipRetry)
		}
		if err != nil {
			return err
		}
		_, _ = db.Exec(ctx, `
			UPDATE interview_reports AS report SET status='running', updated_at=now()
			FROM interview_sessions AS session
			WHERE report.id=$1 AND report.session_id=$2 AND session.id=report.session_id
			  AND session.user_id=$3 AND report.status='pending'`,
			payload.ReportID, payload.SessionID, payload.UserID)

		type turnRow struct {
			ID, Question, Intent, Answer string
			Expected                     []string
		}
		rows, err := db.Query(ctx, `
			SELECT turn.id::text,
			       turn.question,
			       COALESCE(NULLIF(turn.intent, ''), q.intent, NULLIF(turn.capability_key, ''), '动态追问'),
			       COALESCE(turn.answer, ''),
			       CASE WHEN jsonb_array_length(turn.expected_points) > 0
			            THEN turn.expected_points
			            ELSE COALESCE(q.expected_points, '[]'::jsonb)
			       END
			FROM interview_turns AS turn
			LEFT JOIN questions AS q ON q.id = turn.source_question_id
			JOIN interview_sessions AS owner_session ON owner_session.id=turn.session_id
			WHERE turn.session_id=$1 AND owner_session.user_id=$2
			ORDER BY turn.ordinal ASC`, payload.SessionID, payload.UserID)
		if err != nil {
			return failReport(ctx, db, payload, "REPORT_LOAD_FAILED", "无法读取面试回答", err)
		}
		turns := []turnRow{}
		for rows.Next() {
			var item turnRow
			var raw []byte
			if err := rows.Scan(&item.ID, &item.Question, &item.Intent, &item.Answer, &raw); err != nil {
				rows.Close()
				return failReport(ctx, db, payload, "REPORT_LOAD_FAILED", "无法读取面试回答", err)
			}
			_ = json.Unmarshal(raw, &item.Expected)
			turns = append(turns, item)
		}
		rows.Close()
		if len(turns) == 0 {
			return failReport(ctx, db, payload, "INTERVIEW_HAS_NO_TURNS", "该面试没有可生成报告的题目", fmt.Errorf("no turns"))
		}

		facts := []contract.ResumeFact{}
		factRows, err := db.Query(ctx, `
			SELECT fact.fact_type, fact.fact_key, fact.fact_value, fact.source_excerpt, fact.confidence::float8
			FROM resume_facts AS fact
			JOIN interview_sessions AS session ON session.resume_version_id = fact.resume_version_id
			WHERE session.id=$1 AND session.user_id=$2`, payload.SessionID, payload.UserID)
		if err == nil {
			for factRows.Next() {
				var fact contract.ResumeFact
				var raw []byte
				if err := factRows.Scan(&fact.FactType, &fact.FactKey, &raw, &fact.SourceExcerpt, &fact.Confidence); err == nil {
					_ = json.Unmarshal(raw, &fact.FactValue)
					facts = append(facts, fact)
				}
			}
			factRows.Close()
		}

		type scoredTurn struct {
			turn     turnRow
			eval     contract.TurnEvaluation
			score    int
			critique string
			golden   string
			evidence []string
		}
		scored := make([]scoredTurn, len(turns))
		total := 0
		answered := 0
		degraded := chat == nil
		if chat != nil {
			group, groupCtx := errgroup.WithContext(ctx)
			group.SetLimit(2)
			for index, turn := range turns {
				index, turn := index, turn
				group.Go(func() error {
					item := scoredTurn{turn: turn, evidence: []string{}}
					eval, score, _, evalErr := aiworkflow.EvaluateTurn(groupCtx, chat, payload.UserID, payload.TaskID, payload.SessionID, turn.Question, turn.Intent, turn.Answer, turn.Expected, facts)
					if evalErr != nil {
						return evalErr
					}
					item.eval = eval
					item.score = score
					item.critique = strings.Join(eval.Improvements, "；")
					if item.critique == "" && len(eval.Dimensions) > 0 {
						item.critique = eval.Dimensions[0].Reason
					}
					item.golden = eval.GoldenAnswer
					item.evidence = eval.Evidence
					scored[index] = item
					return nil
				})
			}
			if evalErr := group.Wait(); evalErr != nil {
				return failReport(ctx, db, payload, "AI_GENERATION_UNAVAILABLE", "评分生成失败，请重试", evalErr)
			}
		} else {
			for index, turn := range turns {
				item := scoredTurn{turn: turn, evidence: []string{}}
				item.score = degradedScore(turn.Answer)
				item.critique = "当前为降级报告：未调用评分模型，仅记录作答完整性，分数不代表能力评估。"
				item.golden = "建议按 STAR 结合已有材料重答；需要假设的内容应标为示例表达。"
				if strings.TrimSpace(turn.Answer) != "" {
					item.evidence = []string{"用户提交了本题回答"}
				}
				scored[index] = item
			}
		}
		for _, item := range scored {
			if strings.TrimSpace(item.turn.Answer) != "" {
				answered++
			}
			total += item.score
		}
		overall := total / len(scored)
		summaries := []map[string]any{}
		for _, item := range scored {
			summaries = append(summaries, map[string]any{
				"question": item.turn.Question,
				"score":    item.score,
				"empty":    strings.TrimSpace(item.turn.Answer) == "",
			})
		}
		draft := contract.InterviewReportDraft{
			Strengths:    []string{},
			Improvements: []string{"结合材料补充可验证细节"},
			NextSteps:    []string{"针对低分题按 STAR 重练"},
		}
		if chat != nil {
			composed, _, err := aiworkflow.ComposeReport(ctx, chat, payload.UserID, payload.TaskID, payload.SessionID, summaries)
			if err != nil {
				return failReport(ctx, db, payload, "AI_GENERATION_UNAVAILABLE", "报告撰写失败，请重试", err)
			}
			draft = composed
		} else if answered > 0 {
			draft.Strengths = []string{"完成了部分或全部作答，便于后续针对性训练"}
		}
		qualityPassed := answered == len(turns) && overall >= 60
		qualityGate, _ := json.Marshal(map[string]any{
			"passed":         qualityPassed,
			"answered_turns": answered,
			"total_turns":    len(turns),
			"minimum_score":  60,
			"disclaimer":     "评分为训练建议，不作为招聘结论。",
		})
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		failWrite := func(code, summary string, cause error) error {
			_ = tx.Rollback(ctx)
			return failReport(ctx, db, payload, code, summary, cause)
		}
		_, err = tx.Exec(ctx, `DELETE FROM interview_turn_reports WHERE report_id=$1`, payload.ReportID)
		if err != nil {
			return failWrite("REPORT_WRITE_FAILED", "保存报告失败", err)
		}
		status := "completed"
		if degraded {
			status = "degraded"
		}
		updateTag, err := tx.Exec(ctx, `
			UPDATE interview_reports
			SET overall_score=$2, strengths=$3, improvements=$4, next_steps=$5, quality_gate=$6,
			    status=$7, degraded=$8, error_code=NULL, error_summary=NULL, updated_at=now()
			WHERE id=$1 AND session_id=$9`,
			payload.ReportID,
			overall,
			mustJSON(draft.Strengths),
			mustJSON(draft.Improvements),
			mustJSON(draft.NextSteps),
			qualityGate,
			status,
			degraded,
			payload.SessionID,
		)
		if err != nil {
			return failWrite("REPORT_WRITE_FAILED", "保存报告失败", err)
		}
		if updateTag.RowsAffected() != 1 {
			return failWrite("REPORT_TASK_STALE", "报告任务已过期", fmt.Errorf("report update affected %d rows", updateTag.RowsAffected()))
		}
		for _, item := range scored {
			if _, err := tx.Exec(ctx, `
				INSERT INTO interview_turn_reports (report_id, turn_id, score, critique, golden_answer, evidence)
				VALUES ($1,$2,$3,$4,$5,$6)`,
				payload.ReportID, item.turn.ID, item.score, item.critique, item.golden, mustJSON(item.evidence),
			); err != nil {
				return failWrite("REPORT_WRITE_FAILED", "保存报告失败", err)
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE async_tasks SET status='succeeded', progress=100, completed_at=now(), updated_at=now()
			WHERE id=$1 AND user_id=$2 AND status='running'`, payload.TaskID, payload.UserID); err != nil {
			return failWrite("REPORT_WRITE_FAILED", "保存报告失败", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		upsertSkillProfile(ctx, db, payload.UserID, payload.SessionID, draft.Strengths, draft.Improvements)
		return nil
	}
}

func degradedScore(answer string) int {
	if strings.TrimSpace(answer) == "" {
		return 0
	}
	return 50
}

func upsertSkillProfile(ctx context.Context, db *pgxpool.Pool, userID, sessionID string, strengths, gaps []string) {
	if db == nil || strings.TrimSpace(userID) == "" {
		return
	}
	if strengths == nil {
		strengths = []string{}
	}
	if gaps == nil {
		gaps = []string{}
	}
	_, _ = db.Exec(ctx, `
		INSERT INTO user_skill_profiles (user_id, strengths, gaps, notes, source_session_id, updated_at)
		VALUES ($1, $2, $3, '', NULLIF($4, '')::uuid, now())
		ON CONFLICT (user_id) DO UPDATE
		SET strengths = EXCLUDED.strengths,
		    gaps = EXCLUDED.gaps,
		    source_session_id = EXCLUDED.source_session_id,
		    updated_at = now()`,
		userID, mustJSON(strengths), mustJSON(gaps), sessionID,
	)
}

func failReport(ctx context.Context, db *pgxpool.Pool, payload sharedtasks.ReportGeneratePayload, code, summary string, err error) error {
	_, _ = db.Exec(ctx, `
		UPDATE interview_reports AS report
		SET status='failed', error_code=$2, error_summary=$3, updated_at=now()
		FROM interview_sessions AS session
		WHERE report.id=$1 AND report.session_id=$4
		  AND session.id=report.session_id AND session.user_id=$5`,
		payload.ReportID, code, summary, payload.SessionID, payload.UserID)
	_, _ = db.Exec(ctx, `
		UPDATE async_tasks
		SET status='failed', error_code=$2, error_summary=$3, error_message=$4, completed_at=now(), updated_at=now()
		WHERE id=$1 AND user_id=$5 AND status='running'`, payload.TaskID, code, summary, err.Error(), payload.UserID)
	return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
}

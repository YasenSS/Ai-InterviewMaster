package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	"github.com/interviewmaster/interviewmaster/backend/prompts"
	"github.com/jackc/pgx/v5"
)

const (
	questionPromptKey     = "question.generate"
	questionPromptVersion = "v1"
	maxResumePromptRunes  = 30000
	maxJobPromptRunes     = 20000
)

type generatedQuestionSet struct {
	Questions []types.QuestionInput `json:"questions"`
}

func questionsForMaterials(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, resumeID, jobID, targetRole string,
) ([]types.QuestionInput, error) {
	if svcCtx.ChatModel == nil {
		return validateQuestionInputs(generatedQuestions())
	}

	resumeText, jobText, err := loadQuestionGenerationMaterials(ctx, svcCtx, userID, resumeID, jobID)
	if err != nil {
		return nil, err
	}
	return generateQuestionsWithModel(ctx, svcCtx.ChatModel, userID, resumeText, jobText, targetRole)
}

func generateQuestionsWithModel(
	ctx context.Context,
	chatModel platformai.ChatModel,
	userID, resumeText, jobText, targetRole string,
) ([]types.QuestionInput, error) {
	template, err := prompts.Load(questionPromptKey, questionPromptVersion)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(map[string]string{
		"resume":               truncatePromptText(resumeText, maxResumePromptRunes),
		"job_description":      truncatePromptText(jobText, maxJobPromptRunes),
		"target_role":          strings.TrimSpace(targetRole),
		"job_description_note": optionalMaterialNote(jobText),
	})
	if err != nil {
		return nil, err
	}

	runner, err := platformai.NewStructuredRunner[generatedQuestionSet](ctx, chatModel, func(output generatedQuestionSet) error {
		_, err := validateQuestionInputs(output.Questions)
		return err
	})
	if err != nil {
		return nil, err
	}
	temperature := 0.3
	result, err := runner.Invoke(ctx, platformai.StructuredRequest{Generate: platformai.GenerateRequest{
		PromptKey:     questionPromptKey,
		PromptVersion: questionPromptVersion,
		UserID:        userID,
		Messages: []platformai.Message{
			{Role: platformai.RoleSystem, Content: template.System},
			{Role: platformai.RoleUser, Content: template.Task +
				"\n\n输出必须符合以下 JSON Schema：\n" + string(template.JSONSchema) +
				"\n\n<untrusted_data_json>\n" + string(data) + "\n</untrusted_data_json>"},
		},
		SchemaName:  "interview_question_set",
		JSONSchema:  template.JSONSchema,
		MaxTokens:   3000,
		Temperature: &temperature,
	}})
	if err != nil {
		if platformai.IsErrorCode(err, platformai.ErrorOutputInvalid) {
			return nil, apperror.New(
				"AI_OUTPUT_INVALID",
				"AI 生成的题集未通过质量检查，请重试",
				http.StatusServiceUnavailable,
				nil,
				err,
			)
		}
		return nil, apperror.New(
			"AI_GENERATION_UNAVAILABLE",
			"AI 题集生成服务暂时不可用，请稍后重试",
			http.StatusServiceUnavailable,
			nil,
			err,
		)
	}
	return validateQuestionInputs(result.Value.Questions)
}

func loadQuestionGenerationMaterials(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, resumeID, jobID string,
) (string, string, error) {
	var status, resumeText string
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT resume.status::text, COALESCE(version.extracted_text, '')
		FROM resumes AS resume
		LEFT JOIN resume_versions AS version ON version.id = resume.current_version_id
		WHERE resume.id = $1 AND resume.user_id = $2`,
		resumeID,
		userID,
	).Scan(&status, &resumeText)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", resourceNotFound("RESUME_NOT_FOUND", "未找到该简历", err)
	}
	if err != nil {
		return "", "", err
	}
	if status != "completed" || strings.TrimSpace(resumeText) == "" {
		return "", "", conflict("RESUME_NOT_PARSED", "简历解析完成后才能生成题集", nil)
	}

	var jobText string
	if strings.TrimSpace(jobID) != "" {
		err = svcCtx.Database.QueryRow(ctx, `
			SELECT content
			FROM job_descriptions
			WHERE id = $1 AND user_id = $2`,
			jobID,
			userID,
		).Scan(&jobText)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", resourceNotFound("JOB_DESCRIPTION_NOT_FOUND", "未找到该职位描述", err)
		}
		if err != nil {
			return "", "", err
		}
	}
	return resumeText, jobText, nil
}

func truncatePromptText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + fmt.Sprintf("\n[内容因长度限制截断，共 %d 字符]", len(runes))
}

func optionalMaterialNote(jobText string) string {
	if strings.TrimSpace(jobText) == "" {
		return "未提供 JD，只能依据简历和目标岗位出题"
	}
	return "已提供 JD"
}

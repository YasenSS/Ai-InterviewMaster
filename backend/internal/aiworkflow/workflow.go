package aiworkflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
	"github.com/interviewmaster/interviewmaster/backend/prompts"
)

const (
	ResumeExtractKey    = "resume.extract"
	BlueprintKey        = "blueprint.generate"
	QuestionGenerateKey = "question.generate"
	EvaluationKey       = "evaluation.critique"
	ReportComposeKey    = "report.compose"
	PromptV1            = "v1"
	maxResumeRunes      = 30000
	maxJobRunes         = 20000
	maxAnswerRunes      = 8000
)

func invoke[T any](
	ctx context.Context,
	chat platformai.ChatModel,
	key, version, schemaName, userID, taskID, resourceType, resourceID string,
	data any,
	validate platformai.Validator[T],
) (T, platformai.GenerateResponse, error) {
	var zero T
	template, err := prompts.Load(key, version)
	if err != nil {
		return zero, platformai.GenerateResponse{}, err
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return zero, platformai.GenerateResponse{}, err
	}
	runner, err := platformai.NewStructuredRunner[T](ctx, chat, validate)
	if err != nil {
		return zero, platformai.GenerateResponse{}, err
	}
	temperature := 0.2
	result, err := runner.Invoke(ctx, platformai.StructuredRequest{Generate: platformai.GenerateRequest{
		PromptKey:     key,
		PromptVersion: version,
		UserID:        userID,
		TaskID:        taskID,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		Messages: []platformai.Message{
			{Role: platformai.RoleSystem, Content: template.System},
			{Role: platformai.RoleUser, Content: template.Task +
				"\n\n输出必须符合以下 JSON Schema：\n" + string(template.JSONSchema) +
				"\n\n<untrusted_data_json>\n" + string(payload) + "\n</untrusted_data_json>"},
		},
		SchemaName:  schemaName,
		JSONSchema:  template.JSONSchema,
		MaxTokens:   3000,
		Temperature: &temperature,
	}})
	if err != nil {
		return zero, platformai.GenerateResponse{}, err
	}
	return result.Value, result.Response, nil
}

func ExtractResume(ctx context.Context, chat platformai.ChatModel, userID, taskID, versionID, resumeText string) (contract.ResumeExtraction, platformai.GenerateResponse, error) {
	return invoke[contract.ResumeExtraction](
		ctx, chat, ResumeExtractKey, PromptV1, "resume_extraction",
		userID, taskID, "resume_version", versionID,
		map[string]string{"resume": platformai.ClipRunes(resumeText, maxResumeRunes)},
		contract.ValidateResumeExtraction,
	)
}

func GenerateBlueprint(ctx context.Context, chat platformai.ChatModel, userID, taskID, setID, resumeText, jobText, targetRole string, facts []contract.ResumeFact) (contract.InterviewBlueprint, platformai.GenerateResponse, error) {
	value, response, err := invoke[contract.InterviewBlueprint](
		ctx, chat, BlueprintKey, PromptV1, "interview_blueprint",
		userID, taskID, "question_set", setID,
		map[string]any{
			"resume":            platformai.ClipRunes(resumeText, maxResumeRunes),
			"interview_context": platformai.ClipRunes(jobText, maxJobRunes),
			"target_role":       strings.TrimSpace(targetRole),
			"facts":             facts,
		},
		contract.ValidateBlueprint,
	)
	if err != nil {
		return value, response, err
	}
	value.PromptVersion = PromptV1
	value.Model = response.Model
	value.SchemaVersion = "v1"
	return value, response, nil
}

func GenerateQuestions(
	ctx context.Context,
	chat platformai.ChatModel,
	userID, taskID, setID, resumeText, jobText, targetRole string,
	blueprint contract.InterviewBlueprint,
	factIDs map[string]struct{},
) (contract.GeneratedQuestionSet, platformai.GenerateResponse, error) {
	return invoke[contract.GeneratedQuestionSet](
		ctx, chat, QuestionGenerateKey, PromptV1, "interview_question_set",
		userID, taskID, "question_set", setID,
		map[string]any{
			"resume":            platformai.ClipRunes(resumeText, maxResumeRunes),
			"interview_context": platformai.ClipRunes(jobText, maxJobRunes),
			"target_role":       strings.TrimSpace(targetRole),
			"blueprint":         blueprint,
			"fact_ids":          keys(factIDs),
		},
		func(value contract.GeneratedQuestionSet) error {
			return contract.ValidateGeneratedQuestionSet(value, factIDs)
		},
	)
}

func EvaluateTurn(
	ctx context.Context,
	chat platformai.ChatModel,
	userID, taskID, sessionID, question, intent, answer string,
	expected []string,
	facts []contract.ResumeFact,
) (contract.TurnEvaluation, int, platformai.GenerateResponse, error) {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		eval := emptyEvaluation()
		return eval, 0, platformai.GenerateResponse{}, nil
	}
	value, response, err := invoke[contract.TurnEvaluation](
		ctx, chat, EvaluationKey, PromptV1, "turn_evaluation",
		userID, taskID, "interview_session", sessionID,
		map[string]any{
			"question":        question,
			"intent":          intent,
			"expected_points": expected,
			"answer":          platformai.ClipRunes(trimmed, maxAnswerRunes),
			"facts":           facts,
		},
		contract.ValidateTurnEvaluation,
	)
	if err != nil {
		return value, 0, response, err
	}
	return value, contract.AggregateScore(value), response, nil
}

func ComposeReport(
	ctx context.Context,
	chat platformai.ChatModel,
	userID, taskID, sessionID string,
	summaries []map[string]any,
) (contract.InterviewReportDraft, platformai.GenerateResponse, error) {
	return invoke[contract.InterviewReportDraft](
		ctx, chat, ReportComposeKey, PromptV1, "interview_report_draft",
		userID, taskID, "interview_session", sessionID,
		map[string]any{"turn_summaries": summaries},
		contract.ValidateReportDraft,
	)
}

func InputHash(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func UntrustedNote(jobText string) string {
	if strings.TrimSpace(jobText) == "" {
		return "未提供额外面试上下文，只能依据简历和固定岗位出题"
	}
	return "已提供语言、公司和岗位上下文"
}

func emptyEvaluation() contract.TurnEvaluation {
	dims := make([]contract.ScoreDimension, 0, len(contract.DimensionWeights))
	for _, dim := range contract.DimensionWeights {
		dims = append(dims, contract.ScoreDimension{
			Key:    dim.Key,
			Score:  0,
			Weight: dim.Weight,
			Reason: "未提供回答，无法评估该维度；这是证据缺失而不是能力不足。",
		})
	}
	return contract.TurnEvaluation{
		Dimensions:           dims,
		MissingInfo:          []string{"本题未作答"},
		Improvements:         []string{"补充完整回答后再评测"},
		GoldenAnswer:         "建议按 STAR 结合已有材料作答，并标明哪些内容是示例表达。",
		Evidence:             []string{},
		EmptyAnswer:          true,
		InsufficientEvidence: true,
	}
}

func keys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	return result
}

func PreprocessResumeText(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("resume text is empty")
	}
	replaced := strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		return r
	}, text)
	if strings.Count(replaced, "\uFFFD") > 50 {
		return "", fmt.Errorf("resume text appears corrupted")
	}
	return platformai.ClipRunes(replaced, maxResumeRunes), nil
}

package workspace

import (
	"context"
	"strings"
	"testing"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
)

type questionFakeModel struct {
	request platformai.GenerateRequest
}

func (m *questionFakeModel) Generate(_ context.Context, request platformai.GenerateRequest) (platformai.GenerateResponse, error) {
	m.request = request
	return platformai.GenerateResponse{Message: platformai.Message{Role: platformai.RoleAssistant, Content: `{
		"questions":[
			{"ordinal":1,"question":"请介绍支付系统项目。","intent":"项目背景","expected_points":["职责","结果"],"follow_up_hint":"你的个人贡献是什么？"},
			{"ordinal":2,"question":"如何处理高并发？","intent":"技术能力","expected_points":["方案","取舍"],"follow_up_hint":"如何验证效果？"},
			{"ordinal":3,"question":"描述一次故障复盘。","intent":"问题解决","expected_points":["定位","改进"],"follow_up_hint":"如何防止复发？"},
			{"ordinal":4,"question":"描述一次团队分歧。","intent":"协作沟通","expected_points":["沟通","结果"],"follow_up_hint":"你学到了什么？"},
			{"ordinal":5,"question":"为什么选择该岗位？","intent":"岗位动机","expected_points":["理解","规划"],"follow_up_hint":"前三个月做什么？"}
		]
	}`}, Provider: "fake", Model: "fake"}, nil
}

func TestGenerateQuestionsWithModelUsesVersionedPrompt(t *testing.T) {
	model := &questionFakeModel{}
	questions, err := generateQuestionsWithModel(
		context.Background(),
		model,
		"user-id",
		"候选人负责支付系统，并将延迟降低 30%。",
		"需要 Go 和高并发经验。",
		"Go 后端工程师",
	)
	if err != nil {
		t.Fatalf("generateQuestionsWithModel() error = %v", err)
	}
	if len(questions) != 5 || questions[0].Ordinal != 1 {
		t.Fatalf("questions = %#v", questions)
	}
	if model.request.PromptKey != questionPromptKey || model.request.PromptVersion != questionPromptVersion {
		t.Fatalf("prompt identity = %q/%q", model.request.PromptKey, model.request.PromptVersion)
	}
	if model.request.SchemaName == "" || len(model.request.JSONSchema) == 0 {
		t.Fatal("structured output schema was not supplied")
	}
	if !strings.Contains(model.request.Messages[1].Content, "支付系统") || !strings.Contains(model.request.Messages[1].Content, "不可信") && !strings.Contains(model.request.Messages[0].Content, "不可信") {
		t.Fatal("prompt did not preserve data isolation guidance")
	}
}

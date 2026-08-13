package workspace

import (
	"context"
	"strings"
	"testing"

	"github.com/interviewmaster/interviewmaster/backend/internal/aiworkflow"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
)

type questionFakeModel struct {
	request platformai.GenerateRequest
}

func (m *questionFakeModel) Generate(_ context.Context, request platformai.GenerateRequest) (platformai.GenerateResponse, error) {
	m.request = request
	return platformai.GenerateResponse{Message: platformai.Message{Role: platformai.RoleAssistant, Content: `{
		"questions":[
			{"ordinal":1,"question":"请介绍支付系统项目的职责与结果。","intent":"项目背景","expected_points":["职责","结果"],"follow_up_hint":"你的个人贡献是什么？","capability_key":"project","difficulty":"medium","evidence_fact_ids":[],"generic":false},
			{"ordinal":2,"question":"如何处理支付链路中的高并发？","intent":"技术能力","expected_points":["方案","取舍"],"follow_up_hint":"如何验证效果？","capability_key":"systems","difficulty":"hard","evidence_fact_ids":[],"generic":false},
			{"ordinal":3,"question":"描述一次线上故障的定位与复盘。","intent":"问题解决","expected_points":["定位","改进"],"follow_up_hint":"如何防止复发？","capability_key":"problem","difficulty":"medium","evidence_fact_ids":[],"generic":false},
			{"ordinal":4,"question":"描述一次与团队意见不一致的处理方式。","intent":"协作沟通","expected_points":["沟通","结果"],"follow_up_hint":"你学到了什么？","capability_key":"collab","difficulty":"medium","evidence_fact_ids":[],"generic":true},
			{"ordinal":5,"question":"为什么选择这个目标岗位，以及如何补齐差距？","intent":"岗位动机","expected_points":["理解","规划"],"follow_up_hint":"前三个月做什么？","capability_key":"motivation","difficulty":"easy","evidence_fact_ids":[],"generic":true}
		]
	}`}, Provider: "fake", Model: "fake"}, nil
}

func TestGenerateQuestionsUsesVersionedPrompt(t *testing.T) {
	model := &questionFakeModel{}
	set, _, err := aiworkflow.GenerateQuestions(
		context.Background(),
		model,
		"user-id",
		"task-id",
		"set-id",
		"候选人负责支付系统，并将延迟降低 30%。",
		"需要 Go 和高并发经验。",
		"Go 后端工程师",
		contract.InterviewBlueprint{CapabilityKeys: []string{"project", "systems"}, Weights: map[string]int{"project": 50, "systems": 50}, Difficulty: "mixed", QuestionCount: 5, FollowUpBudget: 1, TimeBudgetMinutes: 15, EvidenceScope: []string{"resume"}, SchemaVersion: "v1"},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("GenerateQuestions() error = %v", err)
	}
	if len(set.Questions) != 5 || set.Questions[0].Ordinal != 1 {
		t.Fatalf("questions = %#v", set.Questions)
	}
	if model.request.PromptKey != aiworkflow.QuestionGenerateKey || model.request.PromptVersion != aiworkflow.PromptV1 {
		t.Fatalf("prompt identity = %q/%q", model.request.PromptKey, model.request.PromptVersion)
	}
	if !strings.Contains(model.request.Messages[1].Content, "支付系统") {
		t.Fatal("prompt did not preserve resume data")
	}
}

package contract

import "testing"

func TestAggregateScoreIsDeterministicAndIgnoresEmptyAnswers(t *testing.T) {
	eval := TurnEvaluation{
		Dimensions: []ScoreDimension{
			{Key: "relevance", Score: 80, Weight: 25, Reason: "ok"},
			{Key: "evidence", Score: 60, Weight: 25, Reason: "ok"},
			{Key: "structure", Score: 40, Weight: 15, Reason: "ok"},
			{Key: "depth", Score: 80, Weight: 20, Reason: "ok"},
			{Key: "communication", Score: 100, Weight: 15, Reason: "ok"},
		},
	}
	if got := AggregateScore(eval); got != 72 {
		t.Fatalf("score = %d", got)
	}
	eval.EmptyAnswer = true
	if got := AggregateScore(eval); got != 0 {
		t.Fatalf("empty score = %d", got)
	}
}

func TestValidateGeneratedQuestionSetRejectsUnknownFactIDs(t *testing.T) {
	set := GeneratedQuestionSet{Questions: []GeneratedQuestion{
		validQuestion(1, "q1"), validQuestion(2, "q2"), validQuestion(3, "q3"),
		validQuestion(4, "q4"), validQuestion(5, "q5"),
	}}
	set.Questions[0].EvidenceFactIDs = []string{"missing"}
	if err := ValidateGeneratedQuestionSet(set, map[string]struct{}{"fact-1": {}}); err == nil {
		t.Fatal("unknown fact id accepted")
	}
}

func TestAcceptFollowUpRejectsBudgetAndUnknownAction(t *testing.T) {
	allowed := map[string]struct{}{"project": {}, "systems": {}}
	got := AcceptFollowUp(InterviewerDecision{
		Action: ActionFollowUp, Question: "请补充个人贡献是什么？", CapabilityKey: "project", Reason: "need evidence",
	}, 2, 2, false, "project", allowed)
	if got.Action != ActionNextCapability {
		t.Fatalf("budget not enforced: %#v", got)
	}
	got = AcceptFollowUp(InterviewerDecision{
		Action: ActionFollowUp, Question: "请补充个人贡献是什么？", CapabilityKey: "project", Reason: "need evidence",
	}, 0, 3, true, "project", allowed)
	if got.Action != ActionNextCapability {
		t.Fatalf("nested follow-up accepted: %#v", got)
	}
	got = AcceptFollowUp(InterviewerDecision{
		Action: ActionFollowUp, Question: "这个结果是如何验证的？", CapabilityKey: "project", Reason: "need evidence",
	}, 0, 3, false, "project", allowed)
	if got.Action != ActionFollowUp {
		t.Fatalf("valid follow-up rejected: %#v", got)
	}
}

func validQuestion(ordinal int, text string) GeneratedQuestion {
	return GeneratedQuestion{
		Ordinal:        ordinal,
		Question:       "请说明" + text + "的具体做法是什么？",
		Intent:         "考察项目经历",
		ExpectedPoints: []string{"背景", "结果"},
		FollowUpHint:   "个人贡献是什么？",
		CapabilityKey:  "project",
		Difficulty:     "medium",
	}
}

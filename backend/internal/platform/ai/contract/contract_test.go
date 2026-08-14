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

func TestValidateInterviewerDecisionActionFieldRules(t *testing.T) {
	tests := []struct {
		name     string
		decision InterviewerDecision
		wantErr  bool
	}{
		{
			name:     "follow up",
			decision: InterviewerDecision{Action: ActionFollowUp, Question: "这个指标具体是怎样统计出来的？", CapabilityKey: "project", Reason: "verify evidence"},
		},
		{
			name:     "next capability",
			decision: InterviewerDecision{Action: ActionNextCapability, CapabilityKey: "systems", Reason: "topic covered"},
		},
		{
			name:     "finish",
			decision: InterviewerDecision{Action: ActionFinish, Reason: "all goals covered"},
		},
		{
			name:     "next capability missing key",
			decision: InterviewerDecision{Action: ActionNextCapability, Reason: "move on"},
			wantErr:  true,
		},
		{
			name:     "finish contains question",
			decision: InterviewerDecision{Action: ActionFinish, Question: "extra question", Reason: "done"},
			wantErr:  true,
		},
		{
			name:     "duplicate evidence",
			decision: InterviewerDecision{Action: ActionNextCapability, CapabilityKey: "systems", EvidenceFactIDs: []string{"fact-1", "fact-1"}, Reason: "move on"},
			wantErr:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateInterviewerDecision(test.decision)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateInterviewerDecision() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestAcceptInterviewerDecisionEnforcesBudgetAndDepth(t *testing.T) {
	decision := InterviewerDecision{
		Action: ActionFollowUp, Question: "这个指标具体是怎样统计出来的？", CapabilityKey: "project", Reason: "verify evidence",
	}
	base := InterviewerDecisionPolicy{
		FollowUpBudget:       3,
		MaxFollowUpDepth:     2,
		CurrentCapability:    "project",
		CapabilityKeys:       []string{"project", "systems"},
		CurrentFollowUpDepth: 1,
	}
	if got := AcceptInterviewerDecision(decision, base); got.Action != ActionFollowUp {
		t.Fatalf("second bounded follow-up rejected: %#v", got)
	}

	base.CurrentFollowUpDepth = 2
	if got := AcceptInterviewerDecision(decision, base); got.Action != ActionNextCapability || got.CapabilityKey != "systems" {
		t.Fatalf("depth not enforced: %#v", got)
	}

	base.CurrentFollowUpDepth = 0
	base.FollowUpsUsed = 3
	if got := AcceptInterviewerDecision(decision, base); got.Action != ActionNextCapability {
		t.Fatalf("global budget not enforced: %#v", got)
	}
}

func TestAcceptInterviewerDecisionValidatesCapabilityEvidenceAndFinish(t *testing.T) {
	policy := InterviewerDecisionPolicy{
		CompletedTurns:        2,
		MinimumTurnsForFinish: 3,
		CurrentCapability:     "project",
		CapabilityKeys:        []string{"project", "systems"},
		AllowedEvidenceFactIDs: map[string]struct{}{
			"fact-1": {},
		},
	}
	unknownFact := InterviewerDecision{
		Action: ActionNextCapability, CapabilityKey: "systems", EvidenceFactIDs: []string{"missing"}, Reason: "move on",
	}
	if got := AcceptInterviewerDecision(unknownFact, policy); got.Action != ActionNextCapability || got.Reason != "unknown evidence fact id" {
		t.Fatalf("unknown evidence not rejected: %#v", got)
	}

	unknownCapability := InterviewerDecision{Action: ActionNextCapability, CapabilityKey: "hack", Reason: "move on"}
	if got := AcceptInterviewerDecision(unknownCapability, policy); got.CapabilityKey != "systems" {
		t.Fatalf("unknown capability not replaced: %#v", got)
	}

	finish := InterviewerDecision{Action: ActionFinish, Reason: "coverage complete"}
	if got := AcceptInterviewerDecision(finish, policy); got.Action != ActionNextCapability {
		t.Fatalf("premature finish accepted: %#v", got)
	}
	policy.CompletedTurns = 3
	if got := AcceptInterviewerDecision(finish, policy); got.Action != ActionFinish {
		t.Fatalf("valid finish rejected: %#v", got)
	}
}

func TestNextCapabilityDecisionUsesBlueprintOrder(t *testing.T) {
	got := NextCapabilityDecision("project", []string{"project", "systems", "language"}, "fallback")
	if got.Action != ActionNextCapability || got.CapabilityKey != "systems" || got.Question != "" {
		t.Fatalf("unexpected fallback: %#v", got)
	}
}

func validQuestion(ordinal int, text string) GeneratedQuestion {
	return GeneratedQuestion{
		Ordinal:        ordinal,
		Question:       "请说明 " + text + " 的具体实现方式是什么？",
		Intent:         "考察项目经历",
		ExpectedPoints: []string{"背景", "结果"},
		FollowUpHint:   "个人贡献是什么？",
		CapabilityKey:  "project",
		Difficulty:     "medium",
	}
}

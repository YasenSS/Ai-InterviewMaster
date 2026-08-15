package contract

import "testing"

func testBlueprintV2() InterviewBlueprintV2 {
	value := DefaultInterviewBlueprintV2()
	value.Capabilities = []CapabilityPlan{
		{Key: "language", Label: "语言基础", Weight: 20, TargetEvidence: 2, DifficultyCurve: []string{"medium", "hard"}, Rubric: []string{"原理", "实践"}},
		{Key: "project", Label: "项目深挖", Weight: 30, TargetEvidence: 2, DifficultyCurve: []string{"medium", "hard"}, Rubric: []string{"贡献", "结果"}},
		{Key: "systems", Label: "系统设计", Weight: 30, TargetEvidence: 2, DifficultyCurve: []string{"medium", "hard"}, Rubric: []string{"取舍", "可靠性"}},
		{Key: "incident", Label: "故障处理", Weight: 20, TargetEvidence: 1, DifficultyCurve: []string{"medium"}, Rubric: []string{"定位", "复盘"}},
	}
	return value
}

func TestValidateInterviewBlueprintV2(t *testing.T) {
	value := testBlueprintV2()
	if err := ValidateInterviewBlueprintV2(value); err != nil {
		t.Fatalf("valid blueprint rejected: %v", err)
	}
	value.TargetTurns = 5
	if err := ValidateInterviewBlueprintV2(value); err == nil {
		t.Fatal("model-expanded policy was accepted")
	}
}

func TestValidateNextTurnDecisionV2(t *testing.T) {
	policy := NextTurnDecisionPolicyV2{
		CurrentCapability:      "project",
		AllowedCapabilities:    map[string]struct{}{"project": {}, "systems": {}},
		AllowedEvidenceFactIDs: map[string]struct{}{"fact-1": {}},
		MaxFollowUpDepth:       2,
		MaxFollowUpsTotal:      4,
	}
	decision := NextTurnDecisionV2{
		Action:          ActionDeepen,
		Question:        "这个指标的统计口径是什么？",
		TurnKind:        TurnKindFollowUp,
		CapabilityKey:   "project",
		Intent:          "核实量化结果",
		ExpectedPoints:  []string{"基线", "统计周期"},
		Difficulty:      "hard",
		EvidenceFactIDs: []string{"fact-1"},
		Reason:          "回答包含指标但缺少口径",
		CoverageObservation: CoverageObservation{
			EvidenceQuality: 60,
			Unresolved:      []string{"指标口径"},
		},
	}
	if err := ValidateNextTurnDecisionV2(decision, policy); err != nil {
		t.Fatalf("valid decision rejected: %v", err)
	}
	decision.Question = ""
	if err := ValidateNextTurnDecisionV2(decision, policy); err == nil {
		t.Fatal("empty live question was accepted")
	}
	decision = NextTurnDecisionV2{
		Action:              ActionRecommendFinish,
		Reason:              "覆盖充分",
		EvidenceFactIDs:     []string{},
		CoverageObservation: CoverageObservation{EvidenceQuality: 80},
	}
	if err := ValidateNextTurnDecisionV2(decision, policy); err != nil {
		t.Fatalf("valid finish recommendation rejected: %v", err)
	}
}

func TestEvaluateInterviewPolicyV2(t *testing.T) {
	blueprint := testBlueprintV2()
	if got := EvaluateInterviewPolicyV2(blueprint, InterviewPolicyStateV2{CompletedTurns: 7, CoveredWeight: 100, CriticalCapabilitiesCovered: true, ModelRecommendedFinish: true}); got.ShouldFinish {
		t.Fatalf("finished before minimum: %#v", got)
	}
	if got := EvaluateInterviewPolicyV2(blueprint, InterviewPolicyStateV2{CompletedTurns: 12, CoveredWeight: 90, CriticalCapabilitiesCovered: true}); !got.ShouldFinish {
		t.Fatalf("target coverage did not finish: %#v", got)
	}
	if got := EvaluateInterviewPolicyV2(blueprint, InterviewPolicyStateV2{CompletedTurns: 4, ElapsedMinutes: 30}); !got.ShouldFinish {
		t.Fatalf("time budget did not finish: %#v", got)
	}
}

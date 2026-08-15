package tasks

import (
	"testing"

	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
)

func TestRulePreparationProducesBoundedMaterialsBeyondFiveTurns(t *testing.T) {
	blueprint := ruleBlueprintV2()
	if err := contract.ValidateInterviewBlueprintV2(blueprint); err != nil {
		t.Fatalf("rule blueprint invalid: %v", err)
	}
	materials := ruleMaterialsV2(blueprint, "Go")
	if err := contract.ValidateInterviewMaterials(materials, blueprint, map[string]struct{}{}); err != nil {
		t.Fatalf("rule materials invalid: %v", err)
	}
	items, err := flattenMaterials(blueprint, materials)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 16 {
		t.Fatalf("materials = %d, want 16", len(items))
	}
	if items[0].Kind != "anchor" || items[len(items)-1].Kind != "fallback" {
		t.Fatalf("material kinds were not preserved")
	}
}

func TestCapabilityProgressUnlocksTargetCompletion(t *testing.T) {
	blueprint := ruleBlueprintV2()
	progress := map[string]contract.CapabilityProgress{}
	for _, capability := range blueprint.Capabilities {
		progress[capability.Key] = contract.CapabilityProgress{
			AskedTurns: 2, EvidenceCount: capability.TargetEvidence, EvidenceQuality: 70, CoverageScore: 70,
		}
	}
	if got := coveredWeight(blueprint, progress); got != 100 {
		t.Fatalf("covered weight = %d", got)
	}
	if !criticalCapabilitiesCovered(blueprint, progress) {
		t.Fatal("critical capabilities should be covered")
	}
	result := contract.EvaluateInterviewPolicyV2(blueprint, contract.InterviewPolicyStateV2{
		CompletedTurns: 12, CoveredWeight: 100, CriticalCapabilitiesCovered: true,
	})
	if !result.ShouldFinish {
		t.Fatalf("target policy did not finish: %#v", result)
	}
}

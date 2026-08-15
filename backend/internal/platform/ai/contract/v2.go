package contract

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	BlueprintV2SchemaVersion = "v2"
	StandardInterviewMode    = "standard"

	StandardMinTurns          = 8
	StandardTargetTurns       = 12
	StandardMaxTurns          = 16
	StandardTimeBudgetMinutes = 30
	StandardMaxFollowUpDepth  = 2
	StandardMaxFollowUpsTotal = 4

	ActionDeepen           = "deepen"
	ActionSwitchCapability = "switch_capability"
	ActionRecommendFinish  = "recommend_finish"

	TurnKindMain     = "main"
	TurnKindFollowUp = "follow_up"
)

// CapabilityPlan is one executable coverage target in an interview blueprint.
type CapabilityPlan struct {
	Key             string   `json:"key"`
	Label           string   `json:"label"`
	Weight          int      `json:"weight"`
	TargetEvidence  int      `json:"target_evidence"`
	DifficultyCurve []string `json:"difficulty_curve"`
	Rubric          []string `json:"rubric"`
}

// InterviewBlueprintV2 is the server-bounded policy used by the live
// interviewer. Unlike the legacy blueprint, it never describes a fixed queue.
type InterviewBlueprintV2 struct {
	SchemaVersion     string           `json:"schema_version"`
	Mode              string           `json:"mode"`
	MinTurns          int              `json:"min_turns"`
	TargetTurns       int              `json:"target_turns"`
	MaxTurns          int              `json:"max_turns"`
	TimeBudgetMinutes int              `json:"time_budget_minutes"`
	MaxFollowUpDepth  int              `json:"max_follow_up_depth"`
	MaxFollowUpsTotal int              `json:"max_follow_ups_total"`
	Capabilities      []CapabilityPlan `json:"capabilities"`
	EvidenceScope     []string         `json:"evidence_scope"`
	PromptVersion     string           `json:"prompt_version,omitempty"`
	Model             string           `json:"model,omitempty"`
}

func DefaultInterviewBlueprintV2() InterviewBlueprintV2 {
	return InterviewBlueprintV2{
		SchemaVersion:     BlueprintV2SchemaVersion,
		Mode:              StandardInterviewMode,
		MinTurns:          StandardMinTurns,
		TargetTurns:       StandardTargetTurns,
		MaxTurns:          StandardMaxTurns,
		TimeBudgetMinutes: StandardTimeBudgetMinutes,
		MaxFollowUpDepth:  StandardMaxFollowUpDepth,
		MaxFollowUpsTotal: StandardMaxFollowUpsTotal,
		EvidenceScope:     []string{"resume", "answer", "company_intel"},
	}
}

// ApplyStandardPolicy ensures the model can plan capabilities but cannot
// expand the server-owned time, turn, or follow-up boundaries.
func ApplyStandardPolicy(value InterviewBlueprintV2) InterviewBlueprintV2 {
	value.SchemaVersion = BlueprintV2SchemaVersion
	value.Mode = StandardInterviewMode
	value.MinTurns = StandardMinTurns
	value.TargetTurns = StandardTargetTurns
	value.MaxTurns = StandardMaxTurns
	value.TimeBudgetMinutes = StandardTimeBudgetMinutes
	value.MaxFollowUpDepth = StandardMaxFollowUpDepth
	value.MaxFollowUpsTotal = StandardMaxFollowUpsTotal
	return value
}

func ValidateInterviewBlueprintV2(value InterviewBlueprintV2) error {
	if value.SchemaVersion != BlueprintV2SchemaVersion {
		return fmt.Errorf("schema_version must be %s", BlueprintV2SchemaVersion)
	}
	if value.Mode != StandardInterviewMode {
		return fmt.Errorf("unsupported interview mode")
	}
	if value.MinTurns != StandardMinTurns || value.TargetTurns != StandardTargetTurns || value.MaxTurns != StandardMaxTurns {
		return fmt.Errorf("standard turn policy must be %d/%d/%d", StandardMinTurns, StandardTargetTurns, StandardMaxTurns)
	}
	if value.TimeBudgetMinutes != StandardTimeBudgetMinutes {
		return fmt.Errorf("standard time budget must be %d minutes", StandardTimeBudgetMinutes)
	}
	if value.MaxFollowUpDepth != StandardMaxFollowUpDepth || value.MaxFollowUpsTotal != StandardMaxFollowUpsTotal {
		return fmt.Errorf("standard follow-up policy is invalid")
	}
	if len(value.Capabilities) < 4 || len(value.Capabilities) > 8 {
		return fmt.Errorf("capabilities must contain 4-8 items")
	}
	seen := map[string]struct{}{}
	weight := 0
	for index, item := range value.Capabilities {
		item.Key = strings.TrimSpace(item.Key)
		item.Label = strings.TrimSpace(item.Label)
		if item.Key == "" || item.Label == "" {
			return fmt.Errorf("capability %d key and label are required", index)
		}
		if _, exists := seen[item.Key]; exists {
			return fmt.Errorf("duplicate capability key %q", item.Key)
		}
		seen[item.Key] = struct{}{}
		if item.Weight < 1 || item.Weight > 60 {
			return fmt.Errorf("capability %s weight out of range", item.Key)
		}
		weight += item.Weight
		if item.TargetEvidence < 1 || item.TargetEvidence > 4 {
			return fmt.Errorf("capability %s target_evidence out of range", item.Key)
		}
		if len(item.DifficultyCurve) < 1 || len(item.DifficultyCurve) > 4 {
			return fmt.Errorf("capability %s difficulty curve is required", item.Key)
		}
		for _, difficulty := range item.DifficultyCurve {
			if !validDifficulty(difficulty) {
				return fmt.Errorf("capability %s has invalid difficulty", item.Key)
			}
		}
		if err := validateShortStringList(item.Rubric, 2, 6, 200, "rubric"); err != nil {
			return fmt.Errorf("capability %s: %w", item.Key, err)
		}
	}
	if weight != 100 {
		return fmt.Errorf("capability weights must sum to 100")
	}
	if len(value.EvidenceScope) < 1 || len(value.EvidenceScope) > 6 {
		return fmt.Errorf("evidence_scope is required")
	}
	return nil
}

type CapabilityMaterial struct {
	CapabilityKey     string   `json:"capability_key"`
	ExpectedEvidence  []string `json:"expected_evidence"`
	AnchorQuestions   []string `json:"anchor_questions"`
	FallbackQuestions []string `json:"fallback_questions"`
	EvidenceFactIDs   []string `json:"evidence_fact_ids"`
}

type InterviewMaterials struct {
	SchemaVersion string               `json:"schema_version"`
	Capabilities  []CapabilityMaterial `json:"capabilities"`
}

func ValidateInterviewMaterials(value InterviewMaterials, blueprint InterviewBlueprintV2, allowedFactIDs map[string]struct{}) error {
	if value.SchemaVersion != "v1" {
		return fmt.Errorf("materials schema_version must be v1")
	}
	allowedCapabilities := map[string]struct{}{}
	for _, item := range blueprint.Capabilities {
		allowedCapabilities[item.Key] = struct{}{}
	}
	if len(value.Capabilities) != len(allowedCapabilities) {
		return fmt.Errorf("materials must cover every blueprint capability")
	}
	seen := map[string]struct{}{}
	for _, item := range value.Capabilities {
		item.CapabilityKey = strings.TrimSpace(item.CapabilityKey)
		if _, ok := allowedCapabilities[item.CapabilityKey]; !ok {
			return fmt.Errorf("unknown material capability %q", item.CapabilityKey)
		}
		if _, exists := seen[item.CapabilityKey]; exists {
			return fmt.Errorf("duplicate material capability %q", item.CapabilityKey)
		}
		seen[item.CapabilityKey] = struct{}{}
		if err := validateShortStringList(item.ExpectedEvidence, 2, 6, 300, "expected_evidence"); err != nil {
			return fmt.Errorf("capability %s: %w", item.CapabilityKey, err)
		}
		if err := validateShortStringList(item.AnchorQuestions, 1, 3, 500, "anchor_questions"); err != nil {
			return fmt.Errorf("capability %s: %w", item.CapabilityKey, err)
		}
		if err := validateShortStringList(item.FallbackQuestions, 1, 3, 500, "fallback_questions"); err != nil {
			return fmt.Errorf("capability %s: %w", item.CapabilityKey, err)
		}
		for _, factID := range item.EvidenceFactIDs {
			if _, ok := allowedFactIDs[strings.TrimSpace(factID)]; !ok {
				return fmt.Errorf("material references unknown evidence fact id")
			}
		}
	}
	return nil
}

type CoverageObservation struct {
	EvidenceQuality int      `json:"evidence_quality"`
	Resolved        []string `json:"resolved"`
	Unresolved      []string `json:"unresolved"`
}

type NextTurnDecisionV2 struct {
	Action              string              `json:"action"`
	Question            string              `json:"question"`
	TurnKind            string              `json:"turn_kind"`
	CapabilityKey       string              `json:"capability_key"`
	Intent              string              `json:"intent"`
	ExpectedPoints      []string            `json:"expected_points"`
	Difficulty          string              `json:"difficulty"`
	EvidenceFactIDs     []string            `json:"evidence_fact_ids"`
	Reason              string              `json:"reason"`
	CoverageObservation CoverageObservation `json:"coverage_observation"`
}

type NextTurnDecisionPolicyV2 struct {
	CurrentCapability      string
	AllowedCapabilities    map[string]struct{}
	AllowedEvidenceFactIDs map[string]struct{}
	CurrentFollowUpDepth   int
	MaxFollowUpDepth       int
	FollowUpsUsed          int
	MaxFollowUpsTotal      int
}

func ValidateNextTurnDecisionV2(value NextTurnDecisionV2, policy NextTurnDecisionPolicyV2) error {
	value.Action = strings.TrimSpace(value.Action)
	value.Question = strings.TrimSpace(value.Question)
	value.TurnKind = strings.TrimSpace(value.TurnKind)
	value.CapabilityKey = strings.TrimSpace(value.CapabilityKey)
	value.Intent = strings.TrimSpace(value.Intent)
	value.Difficulty = strings.TrimSpace(value.Difficulty)
	value.Reason = strings.TrimSpace(value.Reason)
	if value.Action != ActionDeepen && value.Action != ActionSwitchCapability && value.Action != ActionRecommendFinish {
		return fmt.Errorf("unsupported next-turn action")
	}
	if value.Reason == "" || utf8.RuneCountInString(value.Reason) > 500 {
		return fmt.Errorf("reason is required and must be at most 500 characters")
	}
	if value.CoverageObservation.EvidenceQuality < 0 || value.CoverageObservation.EvidenceQuality > 100 {
		return fmt.Errorf("evidence_quality out of range")
	}
	if err := validateOptionalStringList(value.CoverageObservation.Resolved, 6, 300, "resolved"); err != nil {
		return err
	}
	if err := validateOptionalStringList(value.CoverageObservation.Unresolved, 6, 300, "unresolved"); err != nil {
		return err
	}
	for _, factID := range value.EvidenceFactIDs {
		if _, ok := policy.AllowedEvidenceFactIDs[strings.TrimSpace(factID)]; !ok {
			return fmt.Errorf("unknown evidence fact id")
		}
	}
	if value.Action == ActionRecommendFinish {
		if value.Question != "" || value.TurnKind != "" || value.CapabilityKey != "" || value.Intent != "" || len(value.ExpectedPoints) != 0 || value.Difficulty != "" {
			return fmt.Errorf("recommend_finish must not contain a next question")
		}
		return nil
	}
	if utf8.RuneCountInString(value.Question) < 2 || utf8.RuneCountInString(value.Question) > 500 {
		return fmt.Errorf("question must be 2-500 characters")
	}
	if utf8.RuneCountInString(value.Intent) < 1 || utf8.RuneCountInString(value.Intent) > 300 {
		return fmt.Errorf("intent is required")
	}
	if err := validateShortStringList(value.ExpectedPoints, 2, 6, 300, "expected_points"); err != nil {
		return err
	}
	if !validDifficulty(value.Difficulty) {
		return fmt.Errorf("difficulty is invalid")
	}
	if _, ok := policy.AllowedCapabilities[value.CapabilityKey]; !ok {
		return fmt.Errorf("unknown capability key")
	}
	switch value.Action {
	case ActionDeepen:
		if value.TurnKind != TurnKindFollowUp {
			return fmt.Errorf("deepen must create a follow_up turn")
		}
		if value.CapabilityKey != strings.TrimSpace(policy.CurrentCapability) {
			return fmt.Errorf("deepen cannot change capability")
		}
		if policy.CurrentFollowUpDepth >= policy.MaxFollowUpDepth || policy.FollowUpsUsed >= policy.MaxFollowUpsTotal {
			return fmt.Errorf("follow-up policy exhausted")
		}
	case ActionSwitchCapability:
		if value.TurnKind != TurnKindMain {
			return fmt.Errorf("switch_capability must create a main turn")
		}
	}
	return nil
}

type CapabilityProgress struct {
	AskedTurns      int      `json:"asked_turns"`
	FollowUpTurns   int      `json:"follow_up_turns"`
	EvidenceCount   int      `json:"evidence_count"`
	EvidenceQuality int      `json:"evidence_quality"`
	CoverageScore   int      `json:"coverage_score"`
	LastDifficulty  string   `json:"last_difficulty"`
	UnresolvedGaps  []string `json:"unresolved_gaps"`
}

type InterviewPolicyStateV2 struct {
	CompletedTurns              int
	ElapsedMinutes              int
	CoveredWeight               int
	CriticalCapabilitiesCovered bool
	ModelRecommendedFinish      bool
}

type InterviewPolicyResultV2 struct {
	ShouldFinish bool
	Reason       string
}

func EvaluateInterviewPolicyV2(blueprint InterviewBlueprintV2, state InterviewPolicyStateV2) InterviewPolicyResultV2 {
	if state.CompletedTurns >= blueprint.MaxTurns {
		return InterviewPolicyResultV2{ShouldFinish: true, Reason: "maximum turns reached"}
	}
	if state.ElapsedMinutes >= blueprint.TimeBudgetMinutes {
		return InterviewPolicyResultV2{ShouldFinish: true, Reason: "time budget reached"}
	}
	if state.CompletedTurns < blueprint.MinTurns {
		return InterviewPolicyResultV2{Reason: "minimum turns not reached"}
	}
	if !state.CriticalCapabilitiesCovered {
		return InterviewPolicyResultV2{Reason: "critical capabilities remain uncovered"}
	}
	if state.CompletedTurns >= blueprint.TargetTurns && state.CoveredWeight >= 80 {
		return InterviewPolicyResultV2{ShouldFinish: true, Reason: "target coverage reached"}
	}
	if state.ModelRecommendedFinish && state.CoveredWeight >= 75 {
		return InterviewPolicyResultV2{ShouldFinish: true, Reason: "finish recommendation accepted"}
	}
	return InterviewPolicyResultV2{Reason: "continue interview"}
}

func validDifficulty(value string) bool {
	switch strings.TrimSpace(value) {
	case "easy", "medium", "hard":
		return true
	default:
		return false
	}
}

func validateShortStringList(values []string, minCount, maxCount, maxRunes int, name string) error {
	if len(values) < minCount || len(values) > maxCount {
		return fmt.Errorf("%s must contain %d-%d items", name, minCount, maxCount)
	}
	return validateOptionalStringList(values, maxCount, maxRunes, name)
}

func validateOptionalStringList(values []string, maxCount, maxRunes int, name string) error {
	if len(values) > maxCount {
		return fmt.Errorf("%s contains too many items", name)
	}
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || utf8.RuneCountInString(value) > maxRunes {
			return fmt.Errorf("%s contains an invalid item", name)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s contains duplicate items", name)
		}
		seen[value] = struct{}{}
	}
	return nil
}

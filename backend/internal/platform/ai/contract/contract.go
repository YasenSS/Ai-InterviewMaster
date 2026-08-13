package contract

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type ResumeExtraction struct {
	Facts []ResumeFact `json:"facts"`
}

type ResumeFact struct {
	FactType      string         `json:"fact_type"`
	FactKey       string         `json:"fact_key"`
	FactValue     map[string]any `json:"fact_value"`
	SourceExcerpt string         `json:"source_excerpt"`
	Confidence    float64        `json:"confidence"`
}

type InterviewBlueprint struct {
	CapabilityKeys    []string       `json:"capability_keys"`
	Weights           map[string]int `json:"weights"`
	Difficulty        string         `json:"difficulty"`
	QuestionCount     int            `json:"question_count"`
	FollowUpBudget    int            `json:"follow_up_budget"`
	TimeBudgetMinutes int            `json:"time_budget_minutes"`
	EvidenceScope     []string       `json:"evidence_scope"`
	SchemaVersion     string         `json:"schema_version"`
	PromptVersion     string         `json:"prompt_version,omitempty"`
	Model             string         `json:"model,omitempty"`
}

type GeneratedQuestionSet struct {
	Questions []GeneratedQuestion `json:"questions"`
}

type GeneratedQuestion struct {
	Ordinal         int      `json:"ordinal"`
	Question        string   `json:"question"`
	Intent          string   `json:"intent"`
	ExpectedPoints  []string `json:"expected_points"`
	FollowUpHint    string   `json:"follow_up_hint"`
	CapabilityKey   string   `json:"capability_key"`
	Difficulty      string   `json:"difficulty"`
	EvidenceFactIDs []string `json:"evidence_fact_ids"`
	Generic         bool     `json:"generic"`
}

type TurnEvaluation struct {
	Dimensions           []ScoreDimension `json:"dimensions"`
	MissingInfo          []string         `json:"missing_info"`
	Improvements         []string         `json:"improvements"`
	GoldenAnswer         string           `json:"golden_answer"`
	Evidence             []string         `json:"evidence"`
	EmptyAnswer          bool             `json:"empty_answer"`
	InsufficientEvidence bool             `json:"insufficient_evidence"`
}

type ScoreDimension struct {
	Key    string `json:"key"`
	Score  int    `json:"score"`
	Reason string `json:"reason"`
	Weight int    `json:"weight"`
}

type InterviewReportDraft struct {
	Strengths    []string `json:"strengths"`
	Improvements []string `json:"improvements"`
	NextSteps    []string `json:"next_steps"`
}

type InterviewerDecision struct {
	Action          string   `json:"action"`
	Question        string   `json:"question"`
	CapabilityKey   string   `json:"capability_key"`
	EvidenceFactIDs []string `json:"evidence_fact_ids"`
	Reason          string   `json:"reason"`
}

const (
	ActionFollowUp       = "follow_up"
	ActionNextCapability = "next_capability"
)

var DimensionWeights = []ScoreDimension{
	{Key: "relevance", Weight: 25},
	{Key: "evidence", Weight: 25},
	{Key: "structure", Weight: 15},
	{Key: "depth", Weight: 20},
	{Key: "communication", Weight: 15},
}

func ValidateResumeExtraction(value ResumeExtraction) error {
	if len(value.Facts) < 1 || len(value.Facts) > 40 {
		return fmt.Errorf("facts count must be 1-40")
	}
	allowed := map[string]struct{}{"skill": {}, "experience": {}, "project": {}, "metric": {}, "education": {}}
	seen := map[string]struct{}{}
	for _, fact := range value.Facts {
		if _, ok := allowed[fact.FactType]; !ok {
			return fmt.Errorf("unsupported fact_type %q", fact.FactType)
		}
		if strings.TrimSpace(fact.FactKey) == "" || utf8.RuneCountInString(fact.FactKey) > 120 {
			return fmt.Errorf("invalid fact_key")
		}
		if utf8.RuneCountInString(fact.SourceExcerpt) < 2 || utf8.RuneCountInString(fact.SourceExcerpt) > 500 {
			return fmt.Errorf("source_excerpt must be 2-500 characters")
		}
		if fact.Confidence < 0 || fact.Confidence > 1 {
			return fmt.Errorf("confidence out of range")
		}
		id := fact.FactType + ":" + fact.FactKey
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate fact %s", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func ValidateBlueprint(value InterviewBlueprint) error {
	if value.QuestionCount < 3 || value.QuestionCount > 8 {
		return fmt.Errorf("question_count must be 3-8")
	}
	if value.FollowUpBudget < 0 || value.FollowUpBudget > 5 {
		return fmt.Errorf("follow_up_budget out of range")
	}
	if value.TimeBudgetMinutes < 10 || value.TimeBudgetMinutes > 60 {
		return fmt.Errorf("time_budget_minutes out of range")
	}
	if len(value.CapabilityKeys) < 2 || len(value.CapabilityKeys) > 8 {
		return fmt.Errorf("capability_keys count must be 2-8")
	}
	switch value.Difficulty {
	case "easy", "medium", "hard", "mixed":
	default:
		return fmt.Errorf("unsupported difficulty")
	}
	total := 0
	for _, key := range value.CapabilityKeys {
		weight := value.Weights[key]
		if weight < 1 {
			return fmt.Errorf("missing weight for %s", key)
		}
		total += weight
	}
	if total != 100 {
		return fmt.Errorf("weights must sum to 100")
	}
	return nil
}

func ValidateGeneratedQuestionSet(value GeneratedQuestionSet, allowedFactIDs map[string]struct{}) error {
	if len(value.Questions) != 5 {
		return fmt.Errorf("expected 5 questions")
	}
	seen := map[string]struct{}{}
	covered := map[string]struct{}{}
	generic := 0
	for index, item := range value.Questions {
		if item.Ordinal != index+1 {
			return fmt.Errorf("ordinals must be continuous from 1")
		}
		if utf8.RuneCountInString(item.Question) < 8 || utf8.RuneCountInString(item.Question) > 2000 {
			return fmt.Errorf("question %d length invalid", item.Ordinal)
		}
		if utf8.RuneCountInString(item.Intent) < 2 || utf8.RuneCountInString(item.Intent) > 1000 {
			return fmt.Errorf("intent %d length invalid", item.Ordinal)
		}
		if len(item.ExpectedPoints) < 2 || len(item.ExpectedPoints) > 5 {
			return fmt.Errorf("expected_points %d count invalid", item.Ordinal)
		}
		norm := strings.ToLower(strings.TrimSpace(item.Question))
		if _, ok := seen[norm]; ok {
			return fmt.Errorf("duplicate question")
		}
		seen[norm] = struct{}{}
		if item.CapabilityKey != "" {
			covered[item.CapabilityKey] = struct{}{}
		}
		switch item.Difficulty {
		case "easy", "medium", "hard":
		default:
			return fmt.Errorf("question %d difficulty invalid", item.Ordinal)
		}
		if item.Generic {
			generic++
		}
		for _, factID := range item.EvidenceFactIDs {
			if allowedFactIDs != nil {
				if _, ok := allowedFactIDs[factID]; !ok {
					return fmt.Errorf("question %d references unknown fact id", item.Ordinal)
				}
			}
		}
	}
	if generic > 2 {
		return fmt.Errorf("too many generic questions")
	}
	return nil
}

func ValidateTurnEvaluation(value TurnEvaluation) error {
	if len(value.Dimensions) != 5 {
		return fmt.Errorf("expected 5 scoring dimensions")
	}
	seen := map[string]struct{}{}
	for _, dim := range value.Dimensions {
		if dim.Score < 0 || dim.Score > 100 {
			return fmt.Errorf("dimension %s score out of range", dim.Key)
		}
		if strings.TrimSpace(dim.Reason) == "" {
			return fmt.Errorf("dimension %s missing reason", dim.Key)
		}
		seen[dim.Key] = struct{}{}
	}
	for _, expected := range DimensionWeights {
		if _, ok := seen[expected.Key]; !ok {
			return fmt.Errorf("missing dimension %s", expected.Key)
		}
	}
	if utf8.RuneCountInString(value.GoldenAnswer) > 4000 {
		return fmt.Errorf("golden_answer too long")
	}
	return nil
}

func ValidateReportDraft(value InterviewReportDraft) error {
	if len(value.Strengths) > 8 || len(value.Improvements) > 8 || len(value.NextSteps) > 8 {
		return fmt.Errorf("report lists too long")
	}
	return nil
}

func ValidateInterviewerDecision(value InterviewerDecision) error {
	switch value.Action {
	case ActionFollowUp, ActionNextCapability:
	default:
		return fmt.Errorf("unsupported interviewer action")
	}
	if utf8.RuneCountInString(value.Reason) < 1 || utf8.RuneCountInString(value.Reason) > 500 {
		return fmt.Errorf("reason length invalid")
	}
	if value.Action == ActionFollowUp {
		if utf8.RuneCountInString(value.Question) < 8 || utf8.RuneCountInString(value.Question) > 2000 {
			return fmt.Errorf("follow-up question length invalid")
		}
		if strings.TrimSpace(value.CapabilityKey) == "" {
			return fmt.Errorf("capability_key required for follow_up")
		}
	}
	if len(value.EvidenceFactIDs) > 8 {
		return fmt.Errorf("too many evidence fact ids")
	}
	return nil
}

// AcceptFollowUp applies blueprint and session guards. The model cannot end
// the interview or exceed the follow-up budget.
func AcceptFollowUp(decision InterviewerDecision, followUpsUsed, followUpBudget int, currentIsFollowUp bool, currentCapability string, allowedCapabilities map[string]struct{}) InterviewerDecision {
	if currentIsFollowUp || followUpBudget-followUpsUsed < 1 {
		return InterviewerDecision{Action: ActionNextCapability, Reason: "follow-up budget exhausted or nested follow-up rejected"}
	}
	if err := ValidateInterviewerDecision(decision); err != nil || decision.Action != ActionFollowUp {
		return InterviewerDecision{Action: ActionNextCapability, Reason: "invalid or next-capability decision"}
	}
	if len(allowedCapabilities) > 0 {
		if _, ok := allowedCapabilities[decision.CapabilityKey]; !ok {
			decision.CapabilityKey = strings.TrimSpace(currentCapability)
		}
	}
	return decision
}

// AggregateScore computes a deterministic overall score from model dimensions.
func AggregateScore(eval TurnEvaluation) int {
	if eval.EmptyAnswer {
		return 0
	}
	weights := map[string]int{}
	for _, dim := range DimensionWeights {
		weights[dim.Key] = dim.Weight
	}
	total := 0
	weight := 0
	for _, dim := range eval.Dimensions {
		w := dim.Weight
		if w == 0 {
			w = weights[dim.Key]
		}
		if w == 0 {
			w = 20
		}
		total += dim.Score * w
		weight += w
	}
	if weight == 0 {
		return 0
	}
	return total / weight
}

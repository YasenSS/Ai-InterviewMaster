package aiworkflow

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
	"github.com/interviewmaster/interviewmaster/backend/prompts"
)

const (
	FollowupKey           = "interview.followup"
	followUpDecideTimeout = 8 * time.Second
)

// NextTurnInput is the complete bounded context for deciding what happens
// after one candidate answer. Profile fields are context data only and are
// always placed inside the prompt's untrusted-data boundary.
type NextTurnInput struct {
	UserID            string
	SessionID         string
	CurrentQuestion   string
	CurrentAnswer     string
	CurrentCapability string

	PrimaryLanguage string
	TargetCompany   string
	TargetRole      string

	// CurrentFollowUpDepth is the number of follow-ups already asked on the
	// current main-question thread. A main question has depth 0.
	CurrentFollowUpDepth int
	// CurrentIsFollowUp remains for source compatibility. New callers should
	// pass CurrentFollowUpDepth; when only this flag is true, depth defaults to 1.
	CurrentIsFollowUp bool
	FollowUpsUsed     int
	FollowUpBudget    int
	MaxFollowUpDepth  int
	CompletedTurns    int
	// MinimumTurnsForFinish defaults to Blueprint.QuestionCount when zero.
	// Set a positive value explicitly when the session has a different target.
	MinimumTurnsForFinish int

	RecentTurns            []map[string]any
	Blueprint              contract.InterviewBlueprint
	AllowedEvidenceFactIDs []string
	Tools                  []platformai.Tool
}

// FollowUpInput is a compatibility alias. The decision node now supports all
// three next-turn actions, so new code should use NextTurnInput.
type FollowUpInput = NextTurnInput

// DecideNextTurn asks the model for a bounded follow_up, next_capability, or
// finish decision and then applies deterministic session-side guards. Model,
// tool, prompt, or structured-output failures safely degrade to next_capability.
func DecideNextTurn(ctx context.Context, chat platformai.ChatModel, input NextTurnInput) (contract.InterviewerDecision, platformai.GenerateResponse, error) {
	policy := decisionPolicy(input)
	fallback := func(reason string) contract.InterviewerDecision {
		return contract.NextCapabilityDecision(input.CurrentCapability, input.Blueprint.CapabilityKeys, reason)
	}
	if chat == nil {
		return fallback("interviewer model unavailable"), platformai.GenerateResponse{}, nil
	}
	if strings.TrimSpace(input.CurrentAnswer) == "" {
		return fallback("empty answer"), platformai.GenerateResponse{}, nil
	}

	template, err := prompts.Load(FollowupKey, PromptV1)
	if err != nil {
		return fallback("interviewer prompt unavailable"), platformai.GenerateResponse{}, nil
	}
	decideCtx, cancel := context.WithTimeout(ctx, followUpDecideTimeout)
	defer cancel()

	allowedEvidence := append([]string(nil), input.AllowedEvidenceFactIDs...)
	sort.Strings(allowedEvidence)
	request := platformai.GenerateRequest{
		PromptKey:     FollowupKey,
		PromptVersion: PromptV1,
		UserID:        input.UserID,
		SessionID:     input.SessionID,
		ResourceType:  "interview_session",
		ResourceID:    input.SessionID,
		Messages: []platformai.Message{
			{Role: platformai.RoleSystem, Content: template.System},
			{Role: platformai.RoleUser, Content: template.Task +
				"\n\n输出必须符合以下 JSON Schema：\n" + string(template.JSONSchema) +
				"\n\n<untrusted_data_json>\n" + mustJSON(map[string]any{
				"primary_language":          platformai.ClipRunes(input.PrimaryLanguage, 100),
				"target_company":            platformai.ClipRunes(input.TargetCompany, 200),
				"target_role":               platformai.ClipRunes(input.TargetRole, 200),
				"current_question":          platformai.ClipRunes(input.CurrentQuestion, 2000),
				"current_answer":            platformai.ClipRunes(input.CurrentAnswer, maxAnswerRunes),
				"current_capability":        strings.TrimSpace(input.CurrentCapability),
				"current_follow_up_depth":   policy.CurrentFollowUpDepth,
				"max_follow_up_depth":       policy.MaxFollowUpDepth,
				"follow_ups_used":           policy.FollowUpsUsed,
				"follow_up_budget":          policy.FollowUpBudget,
				"completed_turns":           policy.CompletedTurns,
				"minimum_turns_for_finish":  policy.MinimumTurnsForFinish,
				"allowed_evidence_fact_ids": allowedEvidence,
				"recent_turns":              input.RecentTurns,
				"blueprint":                 input.Blueprint,
			}) + "\n</untrusted_data_json>"},
		},
		Tools:       input.Tools,
		MaxTokens:   800,
		Temperature: floatPtr(0.2),
	}
	working, _, loopErr := platformai.RunToolLoop(decideCtx, chat, request, platformai.MaxToolCallsPerTurn)
	if loopErr != nil {
		return fallback("interviewer model or tool failure"), platformai.GenerateResponse{}, nil
	}
	working.Tools = nil
	working.JSONSchema = template.JSONSchema
	working.SchemaName = "interviewer_decision"
	runner, err := platformai.NewStructuredRunner(decideCtx, chat, contract.ValidateInterviewerDecision)
	if err != nil {
		return fallback("structured interviewer unavailable"), platformai.GenerateResponse{}, nil
	}
	result, err := runner.Invoke(decideCtx, platformai.StructuredRequest{Generate: working})
	if err != nil {
		return fallback("invalid interviewer decision"), result.Response, nil
	}
	return contract.AcceptInterviewerDecision(result.Value, policy), result.Response, nil
}

// DecideFollowUp is retained for callers using the old name. Unlike the old
// implementation, it can now return next_capability and finish decisions.
func DecideFollowUp(ctx context.Context, chat platformai.ChatModel, input FollowUpInput) (contract.InterviewerDecision, platformai.GenerateResponse, error) {
	return DecideNextTurn(ctx, chat, input)
}

func decisionPolicy(input NextTurnInput) contract.InterviewerDecisionPolicy {
	depth := input.CurrentFollowUpDepth
	if depth <= 0 && input.CurrentIsFollowUp {
		depth = 1
	}
	maxDepth := input.MaxFollowUpDepth
	if maxDepth <= 0 || maxDepth > contract.DefaultMaxFollowUpDepth {
		maxDepth = contract.DefaultMaxFollowUpDepth
	}
	minimumTurns := input.MinimumTurnsForFinish
	if minimumTurns <= 0 {
		minimumTurns = input.Blueprint.QuestionCount
	}
	var allowedEvidence map[string]struct{}
	if input.AllowedEvidenceFactIDs != nil {
		allowedEvidence = make(map[string]struct{}, len(input.AllowedEvidenceFactIDs))
		for _, factID := range input.AllowedEvidenceFactIDs {
			factID = strings.TrimSpace(factID)
			if factID != "" {
				allowedEvidence[factID] = struct{}{}
			}
		}
	}
	return contract.InterviewerDecisionPolicy{
		FollowUpsUsed:          input.FollowUpsUsed,
		FollowUpBudget:         input.FollowUpBudget,
		CurrentFollowUpDepth:   depth,
		MaxFollowUpDepth:       maxDepth,
		CompletedTurns:         input.CompletedTurns,
		MinimumTurnsForFinish:  minimumTurns,
		CurrentCapability:      input.CurrentCapability,
		CapabilityKeys:         input.Blueprint.CapabilityKeys,
		AllowedEvidenceFactIDs: allowedEvidence,
	}
}

func floatPtr(value float64) *float64 { return &value }

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

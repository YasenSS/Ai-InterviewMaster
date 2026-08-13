package aiworkflow

import (
	"context"
	"encoding/json"
	"time"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/contract"
	"github.com/interviewmaster/interviewmaster/backend/prompts"
)

const (
	FollowupKey           = "interview.followup"
	followUpDecideTimeout = 8 * time.Second
)

type FollowUpInput struct {
	UserID            string
	SessionID         string
	CurrentQuestion   string
	CurrentAnswer     string
	CurrentCapability string
	CurrentIsFollowUp bool
	FollowUpsUsed     int
	FollowUpBudget    int
	RecentTurns       []map[string]any
	Blueprint         contract.InterviewBlueprint
	Tools             []platformai.Tool
}

func nextCapability(reason string) contract.InterviewerDecision {
	return contract.InterviewerDecision{
		Action:          contract.ActionNextCapability,
		Question:        "",
		CapabilityKey:   "",
		EvidenceFactIDs: []string{},
		Reason:          reason,
	}
}

func DecideFollowUp(ctx context.Context, chat platformai.ChatModel, input FollowUpInput) (contract.InterviewerDecision, platformai.GenerateResponse, error) {
	allowed := map[string]struct{}{}
	for _, key := range input.Blueprint.CapabilityKeys {
		if key != "" {
			allowed[key] = struct{}{}
		}
	}
	if chat == nil || input.CurrentIsFollowUp || input.FollowUpBudget-input.FollowUpsUsed < 1 {
		return nextCapability("follow-up skipped"), platformai.GenerateResponse{}, nil
	}

	template, err := prompts.Load(FollowupKey, PromptV1)
	if err != nil {
		return nextCapability("prompt unavailable"), platformai.GenerateResponse{}, nil
	}
	decideCtx, cancel := context.WithTimeout(ctx, followUpDecideTimeout)
	defer cancel()

	req := platformai.GenerateRequest{
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
				"current_question":     platformai.ClipRunes(input.CurrentQuestion, 2000),
				"current_answer":       platformai.ClipRunes(input.CurrentAnswer, maxAnswerRunes),
				"current_capability":   input.CurrentCapability,
				"current_is_follow_up": input.CurrentIsFollowUp,
				"follow_ups_used":      input.FollowUpsUsed,
				"follow_up_budget":     input.FollowUpBudget,
				"recent_turns":         input.RecentTurns,
				"blueprint":            input.Blueprint,
			}) + "\n</untrusted_data_json>"},
		},
		Tools:       input.Tools,
		MaxTokens:   800,
		Temperature: floatPtr(0.2),
	}
	working, _, loopErr := platformai.RunToolLoop(decideCtx, chat, req, platformai.MaxToolCallsPerTurn)
	if loopErr != nil {
		return nextCapability("follow-up model or tool failure"), platformai.GenerateResponse{}, nil
	}
	working.Tools = nil
	working.JSONSchema = template.JSONSchema
	working.SchemaName = "interviewer_decision"
	runner, err := platformai.NewStructuredRunner(decideCtx, chat, contract.ValidateInterviewerDecision)
	if err != nil {
		return nextCapability("structured runner unavailable"), platformai.GenerateResponse{}, nil
	}
	result, err := runner.Invoke(decideCtx, platformai.StructuredRequest{Generate: working})
	if err != nil {
		return nextCapability("invalid interviewer decision"), result.Response, nil
	}
	accepted := contract.AcceptFollowUp(
		result.Value,
		input.FollowUpsUsed,
		input.FollowUpBudget,
		input.CurrentIsFollowUp,
		input.CurrentCapability,
		allowed,
	)
	return accepted, result.Response, nil
}

func floatPtr(value float64) *float64 { return &value }

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

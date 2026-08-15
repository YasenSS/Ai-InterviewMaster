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
	BlueprintV2Key       = "blueprint.generate"
	BlueprintV2Version   = "v2"
	MaterialsKey         = "interview.materials"
	MaterialsVersion     = "v1"
	NextTurnV2Key        = "interview.next_turn"
	NextTurnV2Version    = "v2"
	nextTurnV2Timeout    = 30 * time.Second
	maxRecentTurnsV2     = 6
	maxNextTurnToolCalls = 2
)

// GenerateBlueprintV2 plans capability coverage. Turn, time and follow-up
// limits are server-owned standard policy and cannot be expanded by the model.
func GenerateBlueprintV2(
	ctx context.Context,
	chat platformai.ChatModel,
	userID, taskID, setID, resumeText, interviewContext, targetRole string,
	facts []contract.ResumeFact,
) (contract.InterviewBlueprintV2, platformai.GenerateResponse, error) {
	value, response, err := invoke[contract.InterviewBlueprintV2](
		ctx, chat, BlueprintV2Key, BlueprintV2Version, "interview_blueprint_v2",
		userID, taskID, "question_set", setID,
		map[string]any{
			"resume":            platformai.ClipRunes(resumeText, maxResumeRunes),
			"interview_context": platformai.ClipRunes(interviewContext, maxJobRunes),
			"target_role":       strings.TrimSpace(targetRole),
			"facts":             facts,
			"server_policy":     contract.DefaultInterviewBlueprintV2(),
		},
		contract.ValidateInterviewBlueprintV2,
	)
	if err != nil {
		return value, response, err
	}
	value = contract.ApplyStandardPolicy(value)
	value.PromptVersion = BlueprintV2Version
	value.Model = response.Model
	return value, response, contract.ValidateInterviewBlueprintV2(value)
}

// GenerateInterviewMaterials creates internal anchors and safety fallbacks.
// These materials are never a public question set or a fixed interview queue.
func GenerateInterviewMaterials(
	ctx context.Context,
	chat platformai.ChatModel,
	userID, taskID, setID, resumeText, interviewContext, targetRole string,
	blueprint contract.InterviewBlueprintV2,
	factIDs map[string]struct{},
) (contract.InterviewMaterials, platformai.GenerateResponse, error) {
	return invoke[contract.InterviewMaterials](
		ctx, chat, MaterialsKey, MaterialsVersion, "interview_materials",
		userID, taskID, "question_set", setID,
		map[string]any{
			"resume":            platformai.ClipRunes(resumeText, maxResumeRunes),
			"interview_context": platformai.ClipRunes(interviewContext, maxJobRunes),
			"target_role":       strings.TrimSpace(targetRole),
			"blueprint":         blueprint,
			"allowed_fact_ids":  sortedKeys(factIDs),
		},
		func(value contract.InterviewMaterials) error {
			return contract.ValidateInterviewMaterials(value, blueprint, factIDs)
		},
	)
}

type NextTurnInputV2 struct {
	UserID     string
	TaskID     string
	SessionID  string
	DecisionID string
	Language   string
	Company    string
	TargetRole string
	Question   string
	Answer     string
	Capability string

	CurrentFollowUpDepth int
	FollowUpsUsed        int
	CompletedTurns       int
	ElapsedMinutes       int

	Blueprint              contract.InterviewBlueprintV2
	CapabilityProgress     map[string]contract.CapabilityProgress
	RecentTurns            []map[string]any
	AllowedEvidenceFactIDs []string
	Tools                  []platformai.Tool
}

// DecideNextTurnV2 performs one bounded ReAct decision. With no tool calls the
// final structured answer is accepted from the first model invocation.
func DecideNextTurnV2(ctx context.Context, chat platformai.ChatModel, input NextTurnInputV2) (contract.NextTurnDecisionV2, platformai.GenerateResponse, error) {
	template, err := prompts.Load(NextTurnV2Key, NextTurnV2Version)
	if err != nil {
		return contract.NextTurnDecisionV2{}, platformai.GenerateResponse{}, err
	}
	allowedCapabilities := make(map[string]struct{}, len(input.Blueprint.Capabilities))
	for _, capability := range input.Blueprint.Capabilities {
		allowedCapabilities[capability.Key] = struct{}{}
	}
	allowedEvidence := make(map[string]struct{}, len(input.AllowedEvidenceFactIDs))
	for _, factID := range input.AllowedEvidenceFactIDs {
		factID = strings.TrimSpace(factID)
		if factID != "" {
			allowedEvidence[factID] = struct{}{}
		}
	}
	policy := contract.NextTurnDecisionPolicyV2{
		CurrentCapability:      strings.TrimSpace(input.Capability),
		AllowedCapabilities:    allowedCapabilities,
		AllowedEvidenceFactIDs: allowedEvidence,
		CurrentFollowUpDepth:   input.CurrentFollowUpDepth,
		MaxFollowUpDepth:       input.Blueprint.MaxFollowUpDepth,
		FollowUpsUsed:          input.FollowUpsUsed,
		MaxFollowUpsTotal:      input.Blueprint.MaxFollowUpsTotal,
	}
	recent := input.RecentTurns
	if len(recent) > maxRecentTurnsV2 {
		recent = recent[len(recent)-maxRecentTurnsV2:]
	}
	payload, err := json.Marshal(map[string]any{
		"primary_language":          platformai.ClipRunes(input.Language, 100),
		"target_company":            platformai.ClipRunes(input.Company, 200),
		"target_role":               platformai.ClipRunes(input.TargetRole, 200),
		"current_question":          platformai.ClipRunes(input.Question, 2000),
		"current_answer":            platformai.ClipRunes(input.Answer, maxAnswerRunes),
		"current_capability":        input.Capability,
		"current_follow_up_depth":   input.CurrentFollowUpDepth,
		"follow_ups_used":           input.FollowUpsUsed,
		"completed_turns":           input.CompletedTurns,
		"elapsed_minutes":           input.ElapsedMinutes,
		"blueprint":                 input.Blueprint,
		"capability_progress":       input.CapabilityProgress,
		"recent_turns":              recent,
		"allowed_evidence_fact_ids": sortedStringSet(allowedEvidence),
	})
	if err != nil {
		return contract.NextTurnDecisionV2{}, platformai.GenerateResponse{}, err
	}
	decideCtx, cancel := context.WithTimeout(ctx, nextTurnV2Timeout)
	defer cancel()
	temperature := 0.2
	result, err := platformai.RunStructuredToolLoop[contract.NextTurnDecisionV2](
		decideCtx,
		chat,
		platformai.GenerateRequest{
			PromptKey:     NextTurnV2Key,
			PromptVersion: NextTurnV2Version,
			UserID:        input.UserID,
			TaskID:        input.TaskID,
			SessionID:     input.SessionID,
			ResourceType:  "interview_turn_decision",
			ResourceID:    input.DecisionID,
			Messages: []platformai.Message{
				{Role: platformai.RoleSystem, Content: template.System},
				{Role: platformai.RoleUser, Content: template.Task +
					"\n\n输出必须符合以下 JSON Schema：\n" + string(template.JSONSchema) +
					"\n\n<untrusted_data_json>\n" + string(payload) + "\n</untrusted_data_json>"},
			},
			Tools:       input.Tools,
			SchemaName:  "next_turn_decision_v2",
			JSONSchema:  template.JSONSchema,
			MaxTokens:   1200,
			Temperature: &temperature,
		},
		maxNextTurnToolCalls,
		func(value contract.NextTurnDecisionV2) error {
			return contract.ValidateNextTurnDecisionV2(value, policy)
		},
	)
	if err != nil {
		return contract.NextTurnDecisionV2{}, result.Response, err
	}
	return result.Value, result.Response, nil
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedStringSet(values map[string]struct{}) []string {
	return sortedKeys(values)
}

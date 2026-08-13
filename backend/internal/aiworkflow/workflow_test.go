package aiworkflow

import (
	"context"
	"testing"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
)

type scriptedModel struct {
	content string
}

func (m scriptedModel) Generate(_ context.Context, _ platformai.GenerateRequest) (platformai.GenerateResponse, error) {
	return platformai.GenerateResponse{Message: platformai.Message{Role: platformai.RoleAssistant, Content: m.content}, Model: "fake"}, nil
}

func TestEvaluateTurnScoresEmptyAnswersAsZeroWithoutModel(t *testing.T) {
	eval, score, _, err := EvaluateTurn(context.Background(), scriptedModel{}, "u", "t", "s", "q", "intent", "   ", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !eval.EmptyAnswer || score != 0 {
		t.Fatalf("eval = %#v score = %d", eval, score)
	}
}

func TestPreprocessResumeTextRejectsEmpty(t *testing.T) {
	if _, err := PreprocessResumeText("  "); err == nil {
		t.Fatal("empty resume accepted")
	}
}

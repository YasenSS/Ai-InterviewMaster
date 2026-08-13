package workspace

import (
	"testing"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
)

func TestPageParams(t *testing.T) {
	page, pageSize, offset, err := pageParams(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page != 1 || pageSize != 20 || offset != 0 {
		t.Fatalf("defaults = page %d, size %d, offset %d", page, pageSize, offset)
	}
	if _, _, _, err := pageParams(1, 101); err == nil {
		t.Fatal("oversized page was accepted")
	}
}

func TestParseEnumFilterSupportsRepeatedAndCommaSeparatedValues(t *testing.T) {
	values, err := parseEnumFilter(
		"status",
		[]string{"active,completed", "active"},
		map[string]struct{}{"active": {}, "completed": {}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0] != "active" || values[1] != "completed" {
		t.Fatalf("values = %#v", values)
	}
}

func TestValidateQuestionInputsRequiresContinuousOrdinals(t *testing.T) {
	_, err := validateQuestionInputs([]types.QuestionInput{
		{Ordinal: 1, Question: "One?", Intent: "Intent", ExpectedPoints: []string{}},
		{Ordinal: 3, Question: "Three?", Intent: "Intent", ExpectedPoints: []string{}},
	})
	if err == nil {
		t.Fatal("non-continuous ordinals were accepted")
	}
}

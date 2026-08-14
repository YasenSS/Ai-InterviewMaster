package prompts

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoadQuestionGenerateV1(t *testing.T) {
	template, err := Load("question.generate", "v1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if template.System == "" || template.Task == "" || len(template.JSONSchema) == 0 {
		t.Fatalf("template is incomplete: %#v", template)
	}
}

func TestLoadRejectsUnsafePath(t *testing.T) {
	if _, err := Load("../secret", "v1"); err == nil {
		t.Fatal("Load() accepted an unsafe key")
	}
}

func TestLoadRejectsMissingVersion(t *testing.T) {
	if _, err := Load("question.generate", "v999"); err == nil {
		t.Fatal("Load() accepted a missing version")
	}
}

func TestLoadFollowUpV1(t *testing.T) {
	template, err := Load("interview.followup", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if template.System == "" || template.Task == "" {
		t.Fatal("follow-up template incomplete")
	}
	if !bytes.Contains(template.JSONSchema, []byte(`"finish"`)) ||
		!bytes.Contains(template.JSONSchema, []byte(`"next_capability"`)) {
		t.Fatal("next-turn actions missing from schema")
	}
	if !strings.Contains(template.System, "<untrusted_data_json>") ||
		!strings.Contains(template.System, "primary_language") ||
		!strings.Contains(template.System, "target_company") {
		t.Fatal("profile context is not explicitly treated as untrusted data")
	}
}

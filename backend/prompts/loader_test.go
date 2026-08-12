package prompts

import "testing"

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

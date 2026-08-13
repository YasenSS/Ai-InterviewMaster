package tools

import (
	"context"
	"testing"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
)

func TestResumeFactsToolSchemaForbidsUserID(t *testing.T) {
	tool := NewResumeFactsTool(nil, "bound-user")
	if tool.Name() != "lookup_resume_facts" {
		t.Fatalf("name = %s", tool.Name())
	}
	if string(tool.Schema()) == "" || containsUserID(string(tool.Schema())) {
		t.Fatalf("schema leaked tenant field: %s", tool.Schema())
	}
	got, err := tool.Call(context.Background(), `{"query":"x","user_id":"other"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"hits":[],"error":"not_configured"}` {
		t.Fatalf("call = %s", got)
	}
}

func TestStripTenantUsedBeforeLookup(t *testing.T) {
	stripped := platformai.StripTenantArgs(`{"query":"redis","user_id":"someone-else"}`)
	if stripped != `{"query":"redis"}` {
		t.Fatalf("stripped = %s", stripped)
	}
}

func containsUserID(value string) bool {
	return len(value) > 0 && (contains(value, "user_id") || contains(value, "workspace_id"))
}

func contains(value, part string) bool {
	return len(value) >= len(part) && (value == part || len(part) == 0 ||
		(func() bool {
			for i := 0; i+len(part) <= len(value); i++ {
				if value[i:i+len(part)] == part {
					return true
				}
			}
			return false
		})())
}

package ai

import "testing"

func TestClipRunesKeepsPrefixAndRecordsOriginalLength(t *testing.T) {
	got := ClipRunes("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 20)
	if got[:5] != "ABCDE" {
		t.Fatalf("prefix = %q", got)
	}
	if !containsAll(got, "截断", "26") {
		t.Fatalf("clip note missing: %q", got)
	}
}

func TestClipRunesLeavesShortTextUnchanged(t *testing.T) {
	if got := ClipRunes(" short ", 20); got != "short" {
		t.Fatalf("got %q", got)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !contains(value, part) {
			return false
		}
	}
	return true
}

func contains(value, part string) bool {
	return len(value) >= len(part) && (value == part || len(part) == 0 ||
		(len(value) > 0 && (func() bool {
			for i := 0; i+len(part) <= len(value); i++ {
				if value[i:i+len(part)] == part {
					return true
				}
			}
			return false
		})()))
}

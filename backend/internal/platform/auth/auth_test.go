package auth

import (
	"testing"
	"time"
)

func TestUserIDFromTokenVerifiesSignature(t *testing.T) {
	token, err := IssueForSession("user-1", "session-1", "correct-secret", time.Minute)
	if err != nil {
		t.Fatalf("IssueForSession() error = %v", err)
	}
	if got := UserIDFromToken(token, "correct-secret"); got != "user-1" {
		t.Fatalf("verified user ID = %q", got)
	}
	if got := UserIDFromToken(token, "wrong-secret"); got != "" {
		t.Fatalf("wrong-signature user ID = %q", got)
	}
}

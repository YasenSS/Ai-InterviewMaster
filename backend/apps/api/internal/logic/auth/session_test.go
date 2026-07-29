package auth

import (
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	email, ok := normalizeEmail("  User.Name+tag@Example.COM ")
	if !ok {
		t.Fatal("expected email to be valid")
	}
	if email != "user.name+tag@example.com" {
		t.Fatalf("normalized email = %q", email)
	}
	if _, ok := normalizeEmail("not-an-email"); ok {
		t.Fatal("invalid email was accepted")
	}
}

func TestValidatePasswordHonorsBcryptLimit(t *testing.T) {
	if !validatePassword("abcdefgh") {
		t.Fatal("eight-character password was rejected")
	}
	if validatePassword("short") {
		t.Fatal("short password was accepted")
	}
	if validatePassword(strings.Repeat("a", 73)) {
		t.Fatal("password longer than bcrypt's byte limit was accepted")
	}
}

func TestRefreshTokenHashIsDeterministicAndNonReversible(t *testing.T) {
	first := hashToken("refresh-token")
	second := hashToken("refresh-token")
	if first != second {
		t.Fatal("same token produced different hashes")
	}
	if first == "refresh-token" || len(first) != 64 {
		t.Fatalf("unexpected token hash %q", first)
	}
}

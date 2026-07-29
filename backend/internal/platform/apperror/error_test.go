package apperror

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestHTTPResponseDoesNotLeakUnknownError(t *testing.T) {
	status, body := HTTPResponse(context.Background(), errors.New("secret database detail"))
	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d", status)
	}
	response := body.(Response)
	if response.Message == "secret database detail" {
		t.Fatal("internal error detail leaked")
	}
}

func TestHTTPResponseUsesApplicationError(t *testing.T) {
	status, body := HTTPResponse(context.Background(), Unavailable("not ready", map[string]string{"db": "down"}, nil))
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", status)
	}
	response := body.(Response)
	if response.Code != CodeUnavailable {
		t.Fatalf("code = %q", response.Code)
	}
}

func TestHTTPResponseClassifiesMalformedRequest(t *testing.T) {
	status, body := HTTPResponse(context.Background(), errors.New(`field "page" is not set`))
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d", status)
	}
	response := body.(Response)
	if response.Code != CodeValidation {
		t.Fatalf("code = %q", response.Code)
	}
}

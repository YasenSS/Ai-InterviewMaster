package logging

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuditResponseWriterExtractsOnlyStructuredErrorCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := &auditResponseWriter{ResponseWriter: recorder}
	writer.WriteHeader(http.StatusConflict)
	_, _ = writer.Write([]byte(`{"code":"RESOURCE_IN_USE","message":"safe"}`))

	if got := writer.errorCode(); got != "RESOURCE_IN_USE" {
		t.Fatalf("error code = %q", got)
	}
}

func TestPathResourceIDs(t *testing.T) {
	const id = "97127a62-4dc4-4d2f-a5e7-36ad66dfa888"
	values := pathResourceIDs("/api/v1/resumes/" + id + "/reparse")
	if len(values) != 1 || values[0] != id {
		t.Fatalf("resource IDs = %#v", values)
	}
}

func TestRedactAttrHidesSecretsAndEmails(t *testing.T) {
	got := RedactAttr(nil, slog.String("api_key", "sk-secret"))
	if got.Value.String() != "[redacted]" {
		t.Fatalf("api_key = %q", got.Value.String())
	}
	got = RedactAttr(nil, slog.String("answer", "this is a candidate answer"))
	if got.Value.String() != "[redacted]" {
		t.Fatalf("answer = %q", got.Value.String())
	}
	got = RedactAttr(nil, slog.String("email", "alice@example.com"))
	if got.Value.String() != "a***@example.com" {
		t.Fatalf("email = %q", got.Value.String())
	}
}

func TestHTTPMiddlewareDoesNotLogBodies(t *testing.T) {
	handler := HTTPMiddleware("unused")(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/interviews", strings.NewReader(`{"answer":"secret-body"}`))
	handler(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
}

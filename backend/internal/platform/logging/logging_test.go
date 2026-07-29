package logging

import (
	"net/http"
	"net/http/httptest"
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

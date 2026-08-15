package statuscode

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareAppliesCreatedAndSuppressesNoContentBodies(t *testing.T) {
	created := httptest.NewRecorder()
	Middleware(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})(created, httptest.NewRequest(http.MethodPost, "/api/v1/interviews", nil))
	if created.Code != http.StatusAccepted {
		t.Fatalf("accepted status = %d", created.Code)
	}

	deleted := httptest.NewRecorder()
	Middleware(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"unexpected":true}`))
	})(deleted, httptest.NewRequest(http.MethodDelete, "/api/v1/resumes/id", nil))
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", deleted.Code)
	}
	if deleted.Body.Len() != 0 {
		t.Fatalf("204 response body = %q", deleted.Body.String())
	}
}

func TestMiddlewarePreservesErrorStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	Middleware(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "conflict", http.StatusConflict)
	})(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/interviews", nil))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("error status = %d", recorder.Code)
	}
}

package requestid

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewarePreservesValidRequestID(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(Header, "request-1234")
	recorder := httptest.NewRecorder()

	Middleware(func(w http.ResponseWriter, r *http.Request) {
		if got := FromContext(r.Context()); got != "request-1234" {
			t.Fatalf("request ID = %q", got)
		}
	})(recorder, request)

	if got := recorder.Header().Get(Header); got != "request-1234" {
		t.Fatalf("response request ID = %q", got)
	}
}

func TestEnsureAddsCorrelationBeforeMiddlewareChain(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	request = Ensure(recorder, request)

	if got := FromContext(request.Context()); got == "" {
		t.Fatal("request context is missing a generated request ID")
	}
	if got := recorder.Header().Get(Header); got != FromContext(request.Context()) {
		t.Fatalf("response request ID = %q", got)
	}
}

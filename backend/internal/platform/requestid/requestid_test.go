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

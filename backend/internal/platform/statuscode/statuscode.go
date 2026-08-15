package statuscode

import (
	"net/http"
	"strings"
)

// Middleware applies the API contract's success status codes while leaving
// error statuses selected by the central error handler untouched.
func Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writer := &responseWriter{
			ResponseWriter: w,
			successStatus:  successStatus(r.Method, r.URL.Path),
		}
		next(writer, r)
	}
}

type responseWriter struct {
	http.ResponseWriter
	successStatus int
	wroteHeader   bool
	noBody        bool
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if status >= http.StatusOK && status < http.StatusMultipleChoices && w.successStatus != 0 {
		status = w.successStatus
	}
	w.noBody = status == http.StatusNoContent
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.noBody {
		return len(body), nil
	}
	return w.ResponseWriter.Write(body)
}

func successStatus(method, path string) int {
	if method == http.MethodDelete {
		return http.StatusNoContent
	}
	if method != http.MethodPost {
		return 0
	}

	switch path {
	case "/api/v1/auth/logout", "/api/v1/auth/change-password", "/api/v1/me/delete":
		return http.StatusNoContent
	case "/api/v1/auth/register",
		"/api/v1/resumes/uploads":
		return http.StatusCreated
	case "/api/v1/interviews":
		return http.StatusAccepted
	}

	if strings.HasPrefix(path, "/api/v1/resumes/") &&
		(strings.HasSuffix(path, "/reparse") ||
			(strings.Contains(path, "/versions/") && strings.HasSuffix(path, "/complete"))) {
		return http.StatusAccepted
	}
	if strings.HasPrefix(path, "/api/v1/interviews/") &&
		(strings.HasSuffix(path, "/preparation/retry") ||
			strings.HasSuffix(path, "/next-turn/retry") ||
			strings.HasSuffix(path, "/next-turn/fallback") ||
			strings.HasSuffix(path, "/report/retry") ||
			strings.HasSuffix(path, "/skip")) {
		return http.StatusAccepted
	}
	if path == "/api/v1/beta/asr/tasks" {
		return http.StatusAccepted
	}
	return 0
}

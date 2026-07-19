package apperror

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/interviewmaster/interviewmaster/backend/internal/platform/requestid"
)

const (
	CodeInternal    = "INTERNAL_ERROR"
	CodeUnavailable = "SERVICE_UNAVAILABLE"
)

type Error struct {
	Code       string
	Message    string
	HTTPStatus int
	Details    any
	cause      error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.cause
}

func New(code, message string, status int, details any, cause error) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status, Details: details, cause: cause}
}

func Unavailable(message string, details any, cause error) *Error {
	return New(CodeUnavailable, message, http.StatusServiceUnavailable, details, cause)
}

type Response struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Details   any    `json:"details,omitempty"`
}

func HTTPResponse(ctx context.Context, err error) (int, any) {
	requestID := requestid.FromContext(ctx)
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.HTTPStatus, Response{
			Code:      appErr.Code,
			Message:   appErr.Message,
			RequestID: requestID,
			Details:   appErr.Details,
		}
	}

	slog.ErrorContext(ctx, "unhandled application error", "request_id", requestID, "error", err)
	return http.StatusInternalServerError, Response{
		Code:      CodeInternal,
		Message:   "The service could not complete the request.",
		RequestID: requestID,
	}
}

package apperror

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/interviewmaster/interviewmaster/backend/internal/platform/requestid"
)

const (
	CodeInternal        = "INTERNAL_ERROR"
	CodeUnavailable     = "SERVICE_UNAVAILABLE"
	CodeValidation      = "VALIDATION_ERROR"
	CodeUnauthenticated = "AUTH_REQUIRED"
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

func Validation(fields map[string][]string) *Error {
	return New(
		CodeValidation,
		"提交内容有误，请检查后重试",
		http.StatusBadRequest,
		map[string]any{"fields": fields},
		nil,
	)
}

func Unauthorized(code, message string) *Error {
	return New(code, message, http.StatusUnauthorized, nil, nil)
}

func NotFound(code, message string, cause error) *Error {
	return New(code, message, http.StatusNotFound, nil, cause)
}

func Conflict(code, message string, details any) *Error {
	return New(code, message, http.StatusConflict, details, nil)
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

	if isRequestError(err) {
		return http.StatusBadRequest, Response{
			Code:      CodeValidation,
			Message:   "请求格式或参数不正确",
			RequestID: requestID,
		}
	}

	slog.ErrorContext(ctx, "unhandled application error", "request_id", requestID, "error", err)
	return http.StatusInternalServerError, Response{
		Code:      CodeInternal,
		Message:   "The service could not complete the request.",
		RequestID: requestID,
	}
}

func isRequestError(err error) bool {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{
		"field \"",
		"type mismatch for field",
		"invalid character",
		"unexpected eof",
		"cannot unmarshal",
		"parsing ",
		"value out of range",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

package apperror

import (
	"errors"
	"net/http"
)

type Error struct {
	Status  int
	Code    string
	Message string
	Details interface{}
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func New(status int, code, message string, details interface{}) *Error {
	return &Error{
		Status:  status,
		Code:    code,
		Message: message,
		Details: details,
	}
}

func Wrap(err error, status int, code, message string) *Error {
	return &Error{
		Status:  status,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

func Validation(details interface{}) *Error {
	return New(http.StatusBadRequest, "INVALID_REQUEST", "validation error", details)
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, "UNAUTHORIZED", message, nil)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, "FORBIDDEN", message, nil)
}

func NotFound(message string) *Error {
	return New(http.StatusNotFound, "NOT_FOUND", message, nil)
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, "CONFLICT", message, nil)
}

func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, "BAD_REQUEST", message, nil)
}

func Internal(err error) *Error {
	return Wrap(err, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
}

func As(err error) *Error {
	if err == nil {
		return nil
	}

	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal(err)
}

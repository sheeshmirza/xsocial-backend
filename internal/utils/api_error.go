package utils

import (
	"fmt"
	"net/http"
)

type ApiError struct {
	StatusCode    int    `json:"statusCode"`
	Message       string `json:"message"`
	Details       any    `json:"details,omitempty"`
	IsOperational bool   `json:"isOperational"`
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("status %d: %s", e.StatusCode, e.Message)
}

func NewApiError(statusCode int, message string, details ...any) *ApiError {
	var d any = nil
	if len(details) > 0 {
		d = details[0]
	}
	return &ApiError{
		StatusCode:    statusCode,
		Message:       message,
		Details:       d,
		IsOperational: true,
	}
}

func BadRequest(message string, details ...any) *ApiError {
	return NewApiError(http.StatusBadRequest, message, details...)
}

func Unauthorized(message string, details ...any) *ApiError {
	return NewApiError(http.StatusUnauthorized, message, details...)
}

func Forbidden(message string, details ...any) *ApiError {
	return NewApiError(http.StatusForbidden, message, details...)
}

func NotFound(message string, details ...any) *ApiError {
	return NewApiError(http.StatusNotFound, message, details...)
}

func Conflict(message string, details ...any) *ApiError {
	return NewApiError(http.StatusConflict, message, details...)
}

func InternalServerError(message string, details ...any) *ApiError {
	return NewApiError(http.StatusInternalServerError, message, details...)
}

func BadGateway(message string, details ...any) *ApiError {
	return NewApiError(http.StatusBadGateway, message, details...)
}

func GatewayTimeout(message string, details ...any) *ApiError {
	return NewApiError(http.StatusGatewayTimeout, message, details...)
}

package server

import (
	"github.com/gin-gonic/gin"
)

const (
	ErrInvalidRequest  = "invalid_request_error"
	ErrAuthentication  = "authentication_error"
	ErrRateLimit       = "rate_limit_error"
	ErrSessionNotFound = "session_not_found"
	ErrProviderError   = "provider_error"
	ErrToolExecFailed  = "tool_execution_error"
	ErrInternalError   = "internal_error"
	ErrNotFound        = "not_found"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func writeError(c *gin.Context, status int, errType string, message string) {
	c.JSON(status, errorBody{
		Error: errorDetail{
			Type:    errType,
			Message: message,
		},
	})
}

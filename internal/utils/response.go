package utils

import (
	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Meta    any    `json:"meta,omitempty"`
	Details any    `json:"details,omitempty"`
}

func SendSuccess(c *gin.Context, statusCode int, message string, data any, meta ...any) {
	resp := APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
	if len(meta) > 0 && meta[0] != nil {
		resp.Meta = meta[0]
	}
	c.JSON(statusCode, resp)
}

func SendError(c *gin.Context, statusCode int, message string, details any) {
	resp := APIResponse{
		Success: false,
		Message: message,
		Details: details,
	}
	c.JSON(statusCode, resp)
}

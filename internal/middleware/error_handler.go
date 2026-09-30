package middleware

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"xsocial-backend/internal/utils"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC RECOVERED] %v", r)
				c.AbortWithStatusJSON(http.StatusInternalServerError, utils.APIResponse{
					Success: false,
					Message: "Internal server error",
				})
			}
		}()

		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			var apiErr *utils.ApiError
			if errors.As(err, &apiErr) {
				c.JSON(apiErr.StatusCode, utils.APIResponse{
					Success: false,
					Message: apiErr.Message,
					Details: apiErr.Details,
				})
				return
			}

			c.JSON(http.StatusInternalServerError, utils.APIResponse{
				Success: false,
				Message: err.Error(),
			})
		}
	}
}

func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotFound, utils.APIResponse{
			Success: false,
			Message: "Route not found: " + c.Request.URL.Path,
		})
	}
}

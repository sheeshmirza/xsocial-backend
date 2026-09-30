package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

func AuthMiddleware(cfg *config.Config, userRepo *repository.UserRepository, apiKeyService *service.ApiKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, utils.APIResponse{
				Success: false,
				Message: "Missing or invalid Authorization header",
			})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// 1. API Key authentication path
		if strings.HasPrefix(tokenString, "mc_live_") || strings.HasPrefix(tokenString, "xs_live_") {
			if apiKeyService == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, utils.APIResponse{
					Success: false,
					Message: "API key authentication unavailable",
				})
				return
			}

			key, err := apiKeyService.ValidateApiKey(c.Request.Context(), tokenString)
			if err != nil || key == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, utils.APIResponse{
					Success: false,
					Message: "Invalid or expired API key",
				})
				return
			}

			user, err := userRepo.FindByID(c.Request.Context(), key.UserID)
			if err != nil || user == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, utils.APIResponse{
					Success: false,
					Message: "User associated with API key not found",
				})
				return
			}

			c.Set("user", user)
			c.Set("userId", user.ID.Hex())
			c.Set("userOID", user.ID)
			c.Set("apiKey", key)
			c.Set("permissions", key.Permissions)
			if key.BrandID != nil {
				c.Set("brandId", key.BrandID.Hex())
				c.Set("brandOID", *key.BrandID)
			}
			c.Next()
			return
		}

		// 2. JWT bearer token authentication path
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, utils.Unauthorized("Unexpected signing method")
			}
			return []byte(cfg.JWTAccessSecret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, utils.APIResponse{
				Success: false,
				Message: "Invalid or expired token",
			})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, utils.APIResponse{
				Success: false,
				Message: "Invalid token claims",
			})
			return
		}

		sub, ok := claims["sub"].(string)
		if !ok || sub == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, utils.APIResponse{
				Success: false,
				Message: "Invalid token user",
			})
			return
		}

		userOID, err := bson.ObjectIDFromHex(sub)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, utils.APIResponse{
				Success: false,
				Message: "Invalid token user",
			})
			return
		}

		user, err := userRepo.FindByID(c.Request.Context(), userOID)
		if err != nil || user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, utils.APIResponse{
				Success: false,
				Message: "Invalid token user",
			})
			return
		}

		c.Set("user", user)
		c.Set("userId", user.ID.Hex())
		c.Set("userOID", user.ID)
		c.Set("permissions", []string{"full_access"})
		c.Next()
	}
}

func RequirePermission(requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permsVal, exists := c.Get("permissions")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, utils.APIResponse{
				Success: false,
				Message: "Access forbidden: no permissions found",
			})
			return
		}

		perms, ok := permsVal.([]string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, utils.APIResponse{
				Success: false,
				Message: "Access forbidden: invalid permissions format",
			})
			return
		}

		for _, p := range perms {
			if strings.EqualFold(p, "full_access") || strings.EqualFold(p, requiredPermission) {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, utils.APIResponse{
			Success: false,
			Message: "Access forbidden: missing required permission " + requiredPermission,
		})
	}
}

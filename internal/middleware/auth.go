package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

func AuthMiddleware(cfg *config.Config, userRepo *repository.UserRepository) gin.HandlerFunc {
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
		c.Next()
	}
}

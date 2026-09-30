package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/handler"
	"xsocial-backend/internal/middleware"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type RouterDeps struct {
	Config        *config.Config
	UserRepo      *repository.UserRepository
	AuthHandler   *handler.AuthHandler
	UserHandler   *handler.UserHandler
	UploadHandler *handler.UploadHandler
	SocialHandler *handler.SocialHandler
	PostHandler   *handler.PostHandler
}

func SetupRouter(deps *RouterDeps) *gin.Engine {
	if deps.Config.NodeEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CORSMiddleware(deps.Config))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, utils.APIResponse{
			Success: true,
			Message: "Server is healthy",
		})
	})

	api := r.Group(deps.Config.ApiPrefix)
	authMiddleware := middleware.AuthMiddleware(deps.Config, deps.UserRepo)

	// Auth routes
	authRoutes := api.Group("/auth")
	{
		authRoutes.POST("/signup", deps.AuthHandler.Signup)
		authRoutes.POST("/verify-email", deps.AuthHandler.VerifyEmail)
		authRoutes.POST("/login", deps.AuthHandler.Login)
		authRoutes.POST("/refresh", deps.AuthHandler.Refresh)
		authRoutes.POST("/logout", authMiddleware, deps.AuthHandler.Logout)
		authRoutes.POST("/forgot-password", deps.AuthHandler.ForgotPassword)
		authRoutes.POST("/reset-password", deps.AuthHandler.ResetPassword)
	}

	// User routes
	userRoutes := api.Group("/users", authMiddleware)
	{
		userRoutes.GET("/me", deps.UserHandler.GetMe)
		userRoutes.PUT("/me", deps.UserHandler.UpdateMe)
		userRoutes.PUT("/me/change-password", deps.UserHandler.ChangePassword)
		userRoutes.DELETE("/me", deps.UserHandler.DeleteMe)
	}

	// Upload routes
	uploadRoutes := api.Group("/uploads")
	{
		uploadRoutes.POST("/media", authMiddleware, deps.UploadHandler.UploadMedia)
		uploadRoutes.GET("/media/:id", deps.UploadHandler.GetUploadedMedia)
	}

	// Social routes
	socialRoutes := api.Group("/social")
	{
		socialRoutes.GET("/accounts", authMiddleware, deps.SocialHandler.ListAccounts)
		socialRoutes.POST("/connect/:platform", authMiddleware, deps.SocialHandler.ConnectPlatform)
		socialRoutes.GET("/callback/:platform", deps.SocialHandler.OAuthCallback)
		socialRoutes.POST("/refresh-token", authMiddleware, deps.SocialHandler.RefreshSocialToken)
		socialRoutes.DELETE("/:id", authMiddleware, deps.SocialHandler.Disconnect)
	}

	// Post routes
	postRoutes := api.Group("/posts", authMiddleware)
	{
		postRoutes.POST("", deps.PostHandler.CreatePost)
		postRoutes.GET("", deps.PostHandler.ListPosts)
		postRoutes.DELETE("/bulk/published", deps.PostHandler.BulkDeletePublishedPosts)
		postRoutes.DELETE("/bulk/scheduled", deps.PostHandler.BulkDeleteScheduledPosts)
		postRoutes.DELETE("/bulk", deps.PostHandler.BulkDeletePosts)
		postRoutes.POST("/bulk/pause-recurrence", deps.PostHandler.BulkPauseRecurringPosts)
		postRoutes.POST("/bulk/resume-recurrence", deps.PostHandler.BulkResumeRecurringPosts)

		postRoutes.GET("/:id", deps.PostHandler.GetPost)
		postRoutes.PUT("/:id", deps.PostHandler.UpdatePost)
		postRoutes.DELETE("/:id/scheduled", deps.PostHandler.DeleteScheduledPost)
		postRoutes.DELETE("/:id/published", deps.PostHandler.DeletePublishedPost)
		postRoutes.DELETE("/:id", deps.PostHandler.DeletePost)

		postRoutes.POST("/:id/publish", deps.PostHandler.PublishPost)
		postRoutes.POST("/:id/schedule", deps.PostHandler.SchedulePost)
		postRoutes.POST("/:id/pause-recurrence", deps.PostHandler.PauseRecurringPost)
		postRoutes.POST("/:id/resume-recurrence", deps.PostHandler.ResumeRecurringPost)
	}

	r.NoRoute(middleware.NotFoundHandler())

	return r
}

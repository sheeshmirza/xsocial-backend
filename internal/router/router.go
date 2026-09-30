package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/handler"
	"xsocial-backend/internal/middleware"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type RouterDeps struct {
	Config             *config.Config
	UserRepo           *repository.UserRepository
	ApiKeyService      *service.ApiKeyService
	AuthHandler        *handler.AuthHandler
	UserHandler        *handler.UserHandler
	UploadHandler      *handler.UploadHandler
	SocialHandler      *handler.SocialHandler
	PostHandler        *handler.PostHandler
	ApiKeyHandler      *handler.ApiKeyHandler
	BrandHandler       *handler.BrandHandler
	InboxHandler       *handler.InboxHandler
	SmartLinkHandler   *handler.SmartLinkHandler
	AutolistHandler    *handler.AutolistHandler
	BestTimesHandler   *handler.BestTimesHandler
	CompetitorHandler  *handler.CompetitorHandler
	AdCampaignHandler  *handler.AdCampaignHandler
	AnalyticsHandler   *handler.AnalyticsHandler
	ReportHandler      *handler.ReportHandler
	WebhookHandler     *handler.WebhookHandler
	AuditLogHandler    *handler.AuditLogHandler
	MCPHandler         *handler.MCPHandler
}

func SetupRouter(deps *RouterDeps) *gin.Engine {
	if deps.Config.NodeEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CORSMiddleware(deps.Config))

	// Rate limiter: 100 requests/sec with burst of 200
	rateLimiter := middleware.NewRateLimiter(rate.Limit(100), 200)
	r.Use(rateLimiter.Middleware())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, utils.APIResponse{
			Success: true,
			Message: "Metricool / XSocial API server is healthy",
		})
	})

	authMiddleware := middleware.AuthMiddleware(deps.Config, deps.UserRepo, deps.ApiKeyService)

	// Model Context Protocol (MCP) endpoint (compatible with Claude, Cursor, AI agents)
	r.POST("/mcp", authMiddleware, deps.MCPHandler.HandleMCP)
	r.GET("/mcp", authMiddleware, deps.MCPHandler.HandleMCP)

	// Public SmartLink / Link-in-Bio pages
	r.GET("/bio/:slug", deps.SmartLinkHandler.PublicBioPage)
	r.POST("/bio/:slug/click/:buttonId", deps.SmartLinkHandler.PublicTrackClick)

	// Public Web Analytics Tracker pixel and beacon event
	r.GET("/tracker/pixel.png", deps.AnalyticsHandler.PublicTrackerPixel)
	r.POST("/tracker/event", deps.AnalyticsHandler.PublicTrackerEvent)

	// Public Post Review & Approval endpoints (for external clients without account)
	r.GET("/review/:token", deps.PostHandler.PublicReview)
	r.POST("/review/:token/action", deps.PostHandler.PublicReviewAction)

	api := r.Group(deps.Config.ApiPrefix)
	{
		// MCP also available under /api/v1/mcp
		api.POST("/mcp", authMiddleware, deps.MCPHandler.HandleMCP)
		api.GET("/mcp", authMiddleware, deps.MCPHandler.HandleMCP)

		// Public review under api prefix as well
		api.GET("/posts/review/:token", deps.PostHandler.PublicReview)
		api.POST("/posts/review/:token/action", deps.PostHandler.PublicReviewAction)

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

		// API Key management routes
		apiKeyRoutes := api.Group("/api-keys", authMiddleware)
		{
			apiKeyRoutes.POST("", deps.ApiKeyHandler.Create)
			apiKeyRoutes.GET("", deps.ApiKeyHandler.List)
			apiKeyRoutes.DELETE("/:id", deps.ApiKeyHandler.Revoke)
		}

		// Brand & Workspace routes
		brandRoutes := api.Group("/brands", authMiddleware)
		{
			brandRoutes.POST("", deps.BrandHandler.Create)
			brandRoutes.GET("", deps.BrandHandler.List)
			brandRoutes.GET("/:id", deps.BrandHandler.Get)
			brandRoutes.PUT("/:id", deps.BrandHandler.Update)
			brandRoutes.DELETE("/:id", deps.BrandHandler.Delete)
			brandRoutes.POST("/:id/members", deps.BrandHandler.AddMember)
			brandRoutes.GET("/:id/members", deps.BrandHandler.ListMembers)
		}

		// Upload routes
		uploadRoutes := api.Group("/uploads")
		{
			uploadRoutes.POST("/media", authMiddleware, deps.UploadHandler.UploadMedia)
			uploadRoutes.GET("/media/:id", deps.UploadHandler.GetUploadedMedia)
		}

		// Social Handle & OAuth routes
		socialRoutes := api.Group("/social")
		{
			socialRoutes.GET("/accounts", authMiddleware, deps.SocialHandler.ListAccounts)
			socialRoutes.POST("/connect/:platform", authMiddleware, deps.SocialHandler.ConnectPlatform)
			socialRoutes.GET("/callback/:platform", deps.SocialHandler.OAuthCallback)
			socialRoutes.POST("/refresh-token", authMiddleware, deps.SocialHandler.RefreshSocialToken)
			socialRoutes.DELETE("/:id", authMiddleware, deps.SocialHandler.Disconnect)
		}

		// Post & Planner routes
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

			// Approval workflows
			postRoutes.POST("/:id/submit-approval", deps.PostHandler.SubmitForApproval)
			postRoutes.POST("/:id/approve", deps.PostHandler.ApprovePost)
			postRoutes.POST("/:id/reject", deps.PostHandler.RejectPost)
		}

		// Unified Inbox routes
		inboxRoutes := api.Group("/inbox", authMiddleware)
		{
			inboxRoutes.GET("", deps.InboxHandler.List)
			inboxRoutes.GET("/canned-replies", deps.InboxHandler.ListCannedReplies)
			inboxRoutes.POST("/canned-replies", deps.InboxHandler.CreateCannedReply)
			inboxRoutes.DELETE("/canned-replies/:id", deps.InboxHandler.DeleteCannedReply)
			inboxRoutes.GET("/:id", deps.InboxHandler.Get)
			inboxRoutes.POST("/:id/reply", deps.InboxHandler.Reply)
			inboxRoutes.PATCH("/:id/status", deps.InboxHandler.UpdateStatus)
		}

		// SmartLinks (Link-in-bio) routes
		smartLinkRoutes := api.Group("/smartlinks", authMiddleware)
		{
			smartLinkRoutes.POST("", deps.SmartLinkHandler.Create)
			smartLinkRoutes.GET("", deps.SmartLinkHandler.List)
			smartLinkRoutes.PUT("/:id", deps.SmartLinkHandler.Update)
			smartLinkRoutes.DELETE("/:id", deps.SmartLinkHandler.Delete)
			smartLinkRoutes.GET("/:id/analytics", deps.SmartLinkHandler.Analytics)
		}

		// Autolists & RSS Feeds (Evergreen content queues)
		autolistRoutes := api.Group("/autolists", authMiddleware)
		{
			autolistRoutes.POST("", deps.AutolistHandler.Create)
			autolistRoutes.GET("", deps.AutolistHandler.List)
			autolistRoutes.POST("/:id/posts", deps.AutolistHandler.AddPost)
			autolistRoutes.DELETE("/:id", deps.AutolistHandler.Delete)
		}

		rssRoutes := api.Group("/rss-feeds", authMiddleware)
		{
			rssRoutes.POST("", deps.AutolistHandler.CreateRSSFeed)
			rssRoutes.GET("", deps.AutolistHandler.ListRSSFeeds)
			rssRoutes.DELETE("/:id", deps.AutolistHandler.DeleteRSSFeed)
		}

		// Best times to post heatmaps
		plannerRoutes := api.Group("/planner", authMiddleware)
		{
			plannerRoutes.GET("/best-times", deps.BestTimesHandler.GetBestTimes)
		}

		// Competitors tracking
		competitorRoutes := api.Group("/competitors", authMiddleware)
		{
			competitorRoutes.POST("", deps.CompetitorHandler.Add)
			competitorRoutes.GET("", deps.CompetitorHandler.List)
			competitorRoutes.DELETE("/:id", deps.CompetitorHandler.Delete)
			competitorRoutes.POST("/:id/sync", deps.CompetitorHandler.Sync)
		}

		// Ad Campaigns tracking (Google, Meta, TikTok)
		adRoutes := api.Group("/ads", authMiddleware)
		{
			adRoutes.GET("/campaigns", deps.AdCampaignHandler.List)
			adRoutes.GET("/summary", deps.AdCampaignHandler.Summary)
			adRoutes.POST("/sync", deps.AdCampaignHandler.Sync)
		}

		// Web & Social Analytics
		analyticsRoutes := api.Group("/analytics", authMiddleware)
		{
			analyticsRoutes.GET("/web", deps.AnalyticsHandler.WebOverview)
			analyticsRoutes.GET("/social", deps.AnalyticsHandler.SocialOverview)
		}

		// Reports
		reportRoutes := api.Group("/reports", authMiddleware)
		{
			reportRoutes.POST("/generate", deps.ReportHandler.Generate)
			reportRoutes.GET("", deps.ReportHandler.List)
			reportRoutes.GET("/:id/download", deps.ReportHandler.Download)
		}

		// Webhooks
		webhookRoutes := api.Group("/webhooks", authMiddleware)
		{
			webhookRoutes.POST("", deps.WebhookHandler.Create)
			webhookRoutes.GET("", deps.WebhookHandler.List)
			webhookRoutes.DELETE("/:id", deps.WebhookHandler.Delete)
		}

		// Audit Logs
		auditRoutes := api.Group("/audit-logs", authMiddleware)
		{
			auditRoutes.GET("", deps.AuditLogHandler.List)
		}
	}

	return r
}

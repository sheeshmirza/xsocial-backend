package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/database"
	"xsocial-backend/internal/handler"
	"xsocial-backend/internal/platform/instagram"
	"xsocial-backend/internal/platform/linkedin"
	"xsocial-backend/internal/queue"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/router"
	"xsocial-backend/internal/service"
)

func main() {
	cfg := config.LoadConfig()

	// 1. Connect MongoDB
	mongoDB, err := database.ConnectMongoDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect MongoDB: %v", err)
	}

	// 2. Connect Redis
	redisClient := database.ConnectRedis(cfg)

	// 3. Initialize Core Repositories
	userRepo := repository.NewUserRepository(mongoDB.Database)
	rfRepo := repository.NewRefreshTokenRepository(mongoDB.Database)
	handleRepo := repository.NewSocialHandleRepository(mongoDB.Database)
	postRepo := repository.NewPostRepository(mongoDB.Database)
	postActivityRepo := repository.NewPostActivityRepository(mongoDB.Database)

	// 4. Initialize Metricool Repositories
	apiKeyRepo := repository.NewApiKeyRepository(mongoDB.Database)
	brandRepo := repository.NewBrandRepository(mongoDB.Database)
	inboxRepo := repository.NewInboxRepository(mongoDB.Database)
	smartLinkRepo := repository.NewSmartLinkRepository(mongoDB.Database)
	autolistRepo := repository.NewAutolistRepository(mongoDB.Database)
	competitorRepo := repository.NewCompetitorRepository(mongoDB.Database)
	adCampaignRepo := repository.NewAdCampaignRepository(mongoDB.Database)
	analyticsRepo := repository.NewAnalyticsRepository(mongoDB.Database)
	reportRepo := repository.NewReportRepository(mongoDB.Database)
	webhookRepo := repository.NewWebhookRepository(mongoDB.Database)
	auditLogRepo := repository.NewAuditLogRepository(mongoDB.Database)

	// 5. Initialize Dedicated Platform Services & Handlers
	igService := instagram.NewService(cfg.Instagram)
	igHandler := instagram.NewHandler(igService, cfg.JWTAccessSecret)

	liService := linkedin.NewService(cfg.LinkedIn)
	liHandler := linkedin.NewHandler(liService, cfg.JWTAccessSecret)

	// 6. Initialize Core & Platform Domain Services
	tokenService := service.NewTokenService(cfg, rfRepo)
	authService := service.NewAuthService(cfg, userRepo, tokenService)
	userService := service.NewUserService(cfg, userRepo, rfRepo)
	aiService := service.NewAIService(cfg)
	uploadService := service.NewUploadService(cfg)
	socialService := service.NewSocialService(cfg, handleRepo, igService, liService)
	postPublishService := service.NewPostPublishService(handleRepo, socialService, igService, liService)
	liActivityService := service.NewLinkedInActivityService(postRepo, handleRepo, postActivityRepo, liService)

	postService := service.NewPostService(
		postRepo,
		postActivityRepo,
		handleRepo,
		postPublishService,
		aiService,
		liActivityService,
		socialService,
		nil, // queue set below
	)

	// 7. Initialize Metricool Services
	apiKeyService := service.NewApiKeyService(apiKeyRepo, redisClient)
	brandService := service.NewBrandService(brandRepo, userRepo)
	inboxService := service.NewInboxService(inboxRepo, brandRepo, handleRepo)
	smartLinkService := service.NewSmartLinkService(smartLinkRepo, brandRepo)
	autolistService := service.NewAutolistService(autolistRepo, postRepo, postPublishService)
	bestTimesService := service.NewBestTimesService(analyticsRepo, brandRepo)
	competitorService := service.NewCompetitorService(competitorRepo)
	adCampaignService := service.NewAdCampaignService(adCampaignRepo)
	analyticsService := service.NewAnalyticsService(analyticsRepo, brandRepo)
	reportService := service.NewReportService(reportRepo, brandRepo, analyticsRepo)
	webhookService := service.NewWebhookService(webhookRepo)
	auditLogService := service.NewAuditLogService(auditLogRepo)

	// 8. Initialize Queue & Dispatcher
	jobDispatcher := queue.CreateJobDispatcher(postService)
	postQueue := queue.NewPostQueue(redisClient, jobDispatcher)
	postService.SetQueue(postQueue)

	// 9. Requeue Recurring Scheduled Posts on Startup
	ctxStartup, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	postService.RequeueRecurringScheduledPostsOnStartup(ctxStartup)
	cancelStartup()

	// 10. Initialize HTTP Handlers
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService)
	uploadHandler := handler.NewUploadHandler(uploadService)
	socialHandler := handler.NewSocialHandler(socialService, igHandler, liHandler, cfg.JWTAccessSecret)
	postHandler := handler.NewPostHandler(postService)

	apiKeyHandler := handler.NewApiKeyHandler(apiKeyService)
	brandHandler := handler.NewBrandHandler(brandService)
	inboxHandler := handler.NewInboxHandler(inboxService)
	smartLinkHandler := handler.NewSmartLinkHandler(smartLinkService)
	autolistHandler := handler.NewAutolistHandler(autolistService)
	bestTimesHandler := handler.NewBestTimesHandler(bestTimesService)
	competitorHandler := handler.NewCompetitorHandler(competitorService)
	adCampaignHandler := handler.NewAdCampaignHandler(adCampaignService)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsService)
	reportHandler := handler.NewReportHandler(reportService)
	webhookHandler := handler.NewWebhookHandler(webhookService)
	auditLogHandler := handler.NewAuditLogHandler(auditLogService)

	mcpHandler := handler.NewMCPHandler(
		brandService,
		analyticsService,
		bestTimesService,
		competitorService,
		postService,
		postPublishService,
		inboxService,
		smartLinkService,
		adCampaignService,
		reportService,
	)

	// 11. Setup Router with all dependencies
	r := router.SetupRouter(&router.RouterDeps{
		Config:            cfg,
		UserRepo:          userRepo,
		ApiKeyService:     apiKeyService,
		AuthHandler:       authHandler,
		UserHandler:       userHandler,
		UploadHandler:     uploadHandler,
		SocialHandler:     socialHandler,
		PostHandler:       postHandler,
		ApiKeyHandler:     apiKeyHandler,
		BrandHandler:      brandHandler,
		InboxHandler:      inboxHandler,
		SmartLinkHandler:  smartLinkHandler,
		AutolistHandler:   autolistHandler,
		BestTimesHandler:  bestTimesHandler,
		CompetitorHandler: competitorHandler,
		AdCampaignHandler: adCampaignHandler,
		AnalyticsHandler:  analyticsHandler,
		ReportHandler:     reportHandler,
		WebhookHandler:    webhookHandler,
		AuditLogHandler:   auditLogHandler,
		MCPHandler:        mcpHandler,
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	// 12. Graceful Shutdown Listener
	go func() {
		log.Printf("Metricool/XSocial Server listening on port %d (MCP: /mcp, API: %s)", cfg.Port, cfg.ApiPrefix)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	if postQueue != nil {
		postQueue.Close()
	}

	log.Println("Server exited cleanly.")
}

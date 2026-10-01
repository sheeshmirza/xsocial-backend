package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"xsocial-backend/internal/api"
	"xsocial-backend/internal/config"
	"xsocial-backend/internal/database"
	"xsocial-backend/internal/platform/instagram"
	"xsocial-backend/internal/platform/linkedin"
	"xsocial-backend/internal/queue"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/service"
)

func main() {
	cfg := config.LoadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

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

	// 4. Initialize Repositories
	apiKeyRepo := repository.NewApiKeyRepository(mongoDB.Database)
	brandRepo := repository.NewBrandRepository(mongoDB.Database)
	auditLogRepo := repository.NewAuditLogRepository(mongoDB.Database)

	// 5. Initialize Dedicated Platform Services
	igService := instagram.NewService(cfg.Instagram)
	liService := linkedin.NewService(cfg.LinkedIn)

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
		brandRepo,
		postPublishService,
		aiService,
		liActivityService,
		socialService,
		nil, // queue set below
	)

	// 7. Initialize Workspaces and Keys Services
	apiKeyService := service.NewApiKeyService(apiKeyRepo, redisClient, brandRepo)
	brandService := service.NewBrandService(brandRepo, userRepo)
	auditLogService := service.NewAuditLogService(auditLogRepo)

	// 8. Initialize Queue & Dispatcher
	jobDispatcher := queue.CreateJobDispatcher(postService)
	postQueue := queue.NewPostQueue(redisClient, jobDispatcher)
	postService.SetQueue(postQueue)

	// 9. Requeue Recurring Scheduled Posts on Startup
	ctxStartup, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	postService.RequeueRecurringScheduledPostsOnStartup(ctxStartup)
	cancelStartup()

	// 10. Instantiate API Server with social platform routing
	apiServer := api.New(
		cfg,
		logger,
		redisClient,
		userRepo,
		rfRepo,
		handleRepo,
		postRepo,
		postActivityRepo,
		apiKeyRepo,
		brandRepo,
		auditLogRepo,
		tokenService,
		authService,
		userService,
		aiService,
		uploadService,
		socialService,
		postPublishService,
		liActivityService,
		postService,
		apiKeyService,
		brandService,
		auditLogService,
		igService,
		liService,
	)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}

	// 11. Graceful Shutdown Listener
	go func() {
		logger.Info("xSocial API server listening", "port", cfg.Port, "mcp", "/mcp", "api_prefix", cfg.ApiPrefix)
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

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
	"xsocial-backend/internal/database/migrations"
	"xsocial-backend/internal/platform/instagram"
	"xsocial-backend/internal/platform/linkedin"
	"xsocial-backend/internal/queue"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

func main() {
	cfg := config.LoadConfig()
	logger := utils.NewRedactingLogger(os.Stdout, slog.LevelInfo, true)
	slog.SetDefault(logger)

	// 1. Connect MongoDB
	mongoDB, err := database.ConnectMongoDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect MongoDB: %v", err)
	}

	// 1.1 Apply database schema migrations & ensure indexing
	migrator := migrations.NewMigrator(mongoDB.Database)
	migCtx, migCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := migrator.ApplyAll(migCtx); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}
	migCancel()

	// 2. Connect Redis
	redisClient := database.ConnectRedis(cfg)

	// 2.1 Connect PostgreSQL (Relational Engine)
	postgresDB, err := database.ConnectPostgres(cfg)
	if err != nil {
		logger.Warn("postgres_connection_skipped_or_failed", "error", err)
	}

	// 2.2 Connect RabbitMQ (Advanced AMQP Message Broker)
	var rabbitBroker *queue.RabbitMQBroker
	if cfg.RabbitMQURL != "" {
		rabbitBroker, err = queue.NewRabbitMQBroker(cfg.RabbitMQURL)
		if err != nil {
			logger.Warn("rabbitmq_connection_skipped_or_failed", "error", err)
		}
	}

	// 2.3 Connect Qdrant (Vector Database Engine)
	qdrantClient, err := database.NewQdrantClient(cfg)
	if err != nil {
		logger.Warn("qdrant_connection_skipped_or_failed", "error", err)
	}
	vecSearchService := service.NewVectorSearchService(qdrantClient)

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

	// 6. Initialize Multi-Tier Distributed & Micro Cache (L1 Sharded Memory + L2 Redis)
	multiTierCache := database.NewMultiTierCache(redisClient, 1*time.Minute)

	// 7. Initialize Core & Platform Domain Services
	tokenService := service.NewTokenService(cfg, rfRepo)
	authService := service.NewAuthService(cfg, userRepo, tokenService)
	userService := service.NewUserService(cfg, userRepo, rfRepo, redisClient)
	userService.SetCache(multiTierCache)
	aiService := service.NewAIService(cfg)
	uploadService := service.NewUploadService(cfg)
	socialService := service.NewSocialService(cfg, handleRepo, igService, liService)
	socialService.SetCache(multiTierCache)
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
	postService.SetCache(multiTierCache)

	// 8. Initialize Workspaces and Keys Services
	apiKeyService := service.NewApiKeyService(apiKeyRepo, redisClient, brandRepo)
	apiKeyService.SetCache(multiTierCache)
	brandService := service.NewBrandService(brandRepo, userRepo, redisClient)
	brandService.SetCache(multiTierCache)
	auditLogService := service.NewAuditLogService(auditLogRepo)

	// 9. Initialize Queue & Dispatcher
	jobDispatcher := queue.CreateJobDispatcher(postService)
	postQueue := queue.NewPostQueue(redisClient, jobDispatcher)
	postService.SetQueue(postQueue)
	userService.SetDependencies(postRepo, postQueue)
	brandService.SetDependencies(handleRepo)

	// 10. Requeue Recurring Scheduled Posts on Startup
	ctxStartup, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	postService.RequeueRecurringScheduledPostsOnStartup(ctxStartup)
	cancelStartup()

	// 11. Instantiate API Server with clean dependency injection
	apiServer := api.NewWithDeps(api.ServerDeps{
		Config:                  cfg,
		Logger:                  logger,
		Redis:                   redisClient,
		UserRepo:                userRepo,
		RefreshTokenRepo:        rfRepo,
		SocialHandleRepo:        handleRepo,
		PostRepo:                postRepo,
		PostActivityRepo:        postActivityRepo,
		ApiKeyRepo:              apiKeyRepo,
		BrandRepo:               brandRepo,
		AuditLogRepo:            auditLogRepo,
		TokenService:            tokenService,
		AuthService:             authService,
		UserService:             userService,
		AIService:               aiService,
		UploadService:           uploadService,
		SocialService:           socialService,
		PostPublishService:      postPublishService,
		LinkedInActivityService: liActivityService,
		PostService:             postService,
		ApiKeyService:           apiKeyService,
		BrandService:            brandService,
		AuditLogService:         auditLogService,
		VectorSearchService:     vecSearchService,
		Postgres:                postgresDB,
		RabbitMQ:                rabbitBroker,
		Qdrant:                  qdrantClient,
		InstagramService:        igService,
		LinkedInService:         liService,
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// 12. Graceful Shutdown Listener
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

	if rabbitBroker != nil {
		_ = rabbitBroker.Close()
	}

	if postgresDB != nil {
		_ = postgresDB.Close()
	}

	if auditLogService != nil {
		auditLogService.Close()
	}

	if multiTierCache != nil {
		multiTierCache.Close()
	}

	if redisClient != nil && redisClient.Client != nil {
		_ = redisClient.Client.Close()
	}

	if mongoDB != nil && mongoDB.Client != nil {
		_ = mongoDB.Client.Disconnect(ctxShutdown)
	}

	log.Println("Server exited cleanly.")
}

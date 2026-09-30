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

	// 3. Initialize Repositories
	userRepo := repository.NewUserRepository(mongoDB.Database)
	rfRepo := repository.NewRefreshTokenRepository(mongoDB.Database)
	handleRepo := repository.NewSocialHandleRepository(mongoDB.Database)
	postRepo := repository.NewPostRepository(mongoDB.Database)
	postActivityRepo := repository.NewPostActivityRepository(mongoDB.Database)

	// 4. Initialize Dedicated Platform Services & Handlers
	igService := instagram.NewService(cfg.Instagram)
	igHandler := instagram.NewHandler(igService, cfg.JWTAccessSecret)

	liService := linkedin.NewService(cfg.LinkedIn)
	liHandler := linkedin.NewHandler(liService, cfg.JWTAccessSecret)

	// 5. Initialize Core Domain Services
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

	// 6. Initialize Queue & Dispatcher
	jobDispatcher := queue.CreateJobDispatcher(postService)
	postQueue := queue.NewPostQueue(redisClient, jobDispatcher)
	postService.SetQueue(postQueue)

	// 7. Requeue Recurring Scheduled Posts on Startup
	ctxStartup, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	postService.RequeueRecurringScheduledPostsOnStartup(ctxStartup)
	cancelStartup()

	// 8. Initialize HTTP Handlers
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService)
	uploadHandler := handler.NewUploadHandler(uploadService)
	socialHandler := handler.NewSocialHandler(socialService, igHandler, liHandler, cfg.JWTAccessSecret)
	postHandler := handler.NewPostHandler(postService)

	// 9. Setup Router
	r := router.SetupRouter(&router.RouterDeps{
		Config:        cfg,
		UserRepo:      userRepo,
		AuthHandler:   authHandler,
		UserHandler:   userHandler,
		UploadHandler: uploadHandler,
		SocialHandler: socialHandler,
		PostHandler:   postHandler,
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	go func() {
		log.Printf("XS-BE running on port %d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	postQueue.Close()
	log.Println("Server exited cleanly")
}

// Package api exposes the authenticated HTTP API, MCP JSON-RPC server, and public endpoints,
// following the exact routing and architectural patterns of mailhost-backend.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	mathrand "math/rand/v2"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/time/rate"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/database"
	"xsocial-backend/internal/platform/instagram"
	"xsocial-backend/internal/platform/linkedin"
	"xsocial-backend/internal/queue"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type ctxKey struct{}
type permKey struct{}
type roleKey struct{}
type userIDKey struct{}
type brandIDKey struct{}
type reqIDKey struct{}

// Server owns API dependencies, request middleware, and endpoint handlers.
type Server struct {
	cfg          *config.Config
	log          *slog.Logger
	redis        *database.RedisClient
	userRepo     repository.IUserRepository
	rfRepo       repository.IRefreshTokenRepository
	handleRepo   repository.ISocialHandleRepository
	postRepo     repository.IPostRepository
	postActRepo  repository.IPostActivityRepository
	apiKeyRepo   repository.IApiKeyRepository
	brandRepo    repository.IBrandRepository
	auditLogRepo repository.IAuditLogRepository

	// Services
	tokenService    *service.TokenService
	authService     *service.AuthService
	userService     *service.UserService
	aiService       *service.AIService
	uploadService   *service.UploadService
	socialService   *service.SocialService
	postPubService  *service.PostPublishService
	liActService    *service.LinkedInActivityService
	postService     *service.PostService
	apiKeyService   *service.ApiKeyService
	brandService    *service.BrandService
	auditLogService *service.AuditLogService
	vecSearchService *service.VectorSearchService

	// Infrastructure engines
	postgres *database.PostgresDB
	rabbitMQ *queue.RabbitMQBroker
	qdrant   *database.QdrantClient

	// Platforms
	igService *instagram.Service
	liService *linkedin.Service

	// Rate limiting & auth failure tracking
	authMu       sync.Mutex
	authFailures map[string]*authFailureRecord
	limiters     sync.Map // IP/Account -> *rate.Limiter
}

type authFailureRecord struct {
	count     int
	lastSeen  time.Time
	blockedTo time.Time
}

// ServerDeps groups all dependencies for Server to establish a clean DI pattern.
type ServerDeps struct {
	Config                  *config.Config
	Logger                  *slog.Logger
	Redis                   *database.RedisClient
	UserRepo                repository.IUserRepository
	RefreshTokenRepo        repository.IRefreshTokenRepository
	SocialHandleRepo        repository.ISocialHandleRepository
	PostRepo                repository.IPostRepository
	PostActivityRepo        repository.IPostActivityRepository
	ApiKeyRepo              repository.IApiKeyRepository
	BrandRepo               repository.IBrandRepository
	AuditLogRepo            repository.IAuditLogRepository
	TokenService            *service.TokenService
	AuthService             *service.AuthService
	UserService             *service.UserService
	AIService               *service.AIService
	UploadService           *service.UploadService
	SocialService           *service.SocialService
	PostPublishService      *service.PostPublishService
	LinkedInActivityService *service.LinkedInActivityService
	PostService             *service.PostService
	ApiKeyService           *service.ApiKeyService
	BrandService            *service.BrandService
	AuditLogService         *service.AuditLogService
	VectorSearchService     *service.VectorSearchService
	Postgres                *database.PostgresDB
	RabbitMQ                *queue.RabbitMQBroker
	Qdrant                  *database.QdrantClient
	InstagramService        *instagram.Service
	LinkedInService         *linkedin.Service
}

func NewWithDeps(deps ServerDeps) *Server {
	logger := deps.Logger
	if logger == nil {
		logger = utils.NewRedactingLogger(nil, slog.LevelInfo, true)
	}
	return &Server{
		cfg:              deps.Config,
		log:              logger,
		redis:            deps.Redis,
		userRepo:         deps.UserRepo,
		rfRepo:           deps.RefreshTokenRepo,
		handleRepo:       deps.SocialHandleRepo,
		postRepo:         deps.PostRepo,
		postActRepo:      deps.PostActivityRepo,
		apiKeyRepo:       deps.ApiKeyRepo,
		brandRepo:        deps.BrandRepo,
		auditLogRepo:     deps.AuditLogRepo,
		tokenService:     deps.TokenService,
		authService:      deps.AuthService,
		userService:      deps.UserService,
		aiService:        deps.AIService,
		uploadService:    deps.UploadService,
		socialService:    deps.SocialService,
		postPubService:   deps.PostPublishService,
		liActService:     deps.LinkedInActivityService,
		postService:      deps.PostService,
		apiKeyService:    deps.ApiKeyService,
		brandService:     deps.BrandService,
		auditLogService:  deps.AuditLogService,
		vecSearchService: deps.VectorSearchService,
		postgres:         deps.Postgres,
		rabbitMQ:         deps.RabbitMQ,
		qdrant:           deps.Qdrant,
		igService:        deps.InstagramService,
		liService:        deps.LinkedInService,
		authFailures:     make(map[string]*authFailureRecord),
	}
}

func New(
	cfg *config.Config,
	log *slog.Logger,
	redis *database.RedisClient,
	userRepo repository.IUserRepository,
	rfRepo repository.IRefreshTokenRepository,
	handleRepo repository.ISocialHandleRepository,
	postRepo repository.IPostRepository,
	postActRepo repository.IPostActivityRepository,
	apiKeyRepo repository.IApiKeyRepository,
	brandRepo repository.IBrandRepository,
	auditLogRepo repository.IAuditLogRepository,
	tokenService *service.TokenService,
	authService *service.AuthService,
	userService *service.UserService,
	aiService *service.AIService,
	uploadService *service.UploadService,
	socialService *service.SocialService,
	postPubService *service.PostPublishService,
	liActService *service.LinkedInActivityService,
	postService *service.PostService,
	apiKeyService *service.ApiKeyService,
	brandService *service.BrandService,
	auditLogService *service.AuditLogService,
	igService *instagram.Service,
	liService *linkedin.Service,
) *Server {
	return NewWithDeps(ServerDeps{
		Config:                  cfg,
		Logger:                  log,
		Redis:                   redis,
		UserRepo:                userRepo,
		RefreshTokenRepo:        rfRepo,
		SocialHandleRepo:        handleRepo,
		PostRepo:                postRepo,
		PostActivityRepo:        postActRepo,
		ApiKeyRepo:              apiKeyRepo,
		BrandRepo:               brandRepo,
		AuditLogRepo:            auditLogRepo,
		TokenService:            tokenService,
		AuthService:             authService,
		UserService:             userService,
		AIService:               aiService,
		UploadService:           uploadService,
		SocialService:           socialService,
		PostPublishService:      postPubService,
		LinkedInActivityService: liActService,
		PostService:             postService,
		ApiKeyService:           apiKeyService,
		BrandService:            brandService,
		AuditLogService:         auditLogService,
		InstagramService:        igService,
		LinkedInService:         liService,
	})
}

// Handler constructs the complete HTTP handler matching the routing architecture of mailhost-backend
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /health", s.health)

	// Public Tracking, Bio Landing, External Reviews, and Public Auth Endpoints
	publicEndpoints := map[string]http.HandlerFunc{
		// Public Auth
		"POST /v1/auth/signup":          s.signupUser,
		"POST /v1/auth/verify-email":    s.verifyEmail,
		"POST /v1/auth/login":           s.loginUser,
		"POST /v1/auth/refresh":         s.refreshToken,
		"POST /v1/auth/forgot-password": s.forgotPassword,
		"POST /v1/auth/reset-password":  s.resetPassword,
		"POST /v1/auth/logout":          s.logoutUser,

		// Public Post Review & Approval (External Client Review)
		"GET /v1/posts/review/{token}":         s.publicReview,
		"POST /v1/posts/review/{token}/action": s.publicReviewAction,

		// Public Media Access
		"GET /v1/uploads/media/{id}": s.getUploadedMedia,

		// OAuth Callback
		"GET /v1/social/callback/{platform}": s.oauthCallback,
	}

	for pattern, h := range publicEndpoints {
		mux.HandleFunc(pattern, h)
		if strings.Contains(pattern, " /v1/") {
			aliasPattern := strings.Replace(pattern, " /v1/", " /", 1)
			mux.HandleFunc(aliasPattern, h)
			apiAliasPattern := strings.Replace(pattern, " /v1/", " /api/v1/", 1)
			mux.HandleFunc(apiAliasPattern, h)
		}
	}

	// Authenticated routes (Wrapped with s.authed)
	routes := map[string]http.HandlerFunc{
		// Model Context Protocol (MCP) Server JSON-RPC 2.0 endpoint.
		// Authentication is enforced inside handleMCP so OAuth discovery and the
		// initial tools/list exchange remain reachable before the user links xSocial.
		"POST /mcp": s.handleMCP,
		"GET /mcp":  s.handleMCP,

		// MCP OAuth 2.1 discovery and authorization endpoints.
		"GET /.well-known/oauth-protected-resource":   s.mcpProtectedResourceMetadata,
		"GET /.well-known/oauth-authorization-server": s.mcpAuthorizationServerMetadata,
		"GET /oauth/authorize":                        s.mcpAuthorize,
		"POST /oauth/authorize":                       s.mcpAuthorize,
		"POST /oauth/token":                           s.mcpToken,

		// Auth & User Management
		"GET /v1/users/me":              s.getCurrentUser,
		"PUT /v1/users/me":              s.updateCurrentUser,
		"PATCH /v1/users/me":            s.updateCurrentUser,
		"PUT /v1/users/change-password": s.changePassword,
		"DELETE /v1/users/me":           s.deleteCurrentUser,

		// API Keys
		"POST /v1/api-keys":        s.createAPIKey,
		"GET /v1/api-keys":         s.listAPIKeys,
		"DELETE /v1/api-keys/{id}": s.deleteAPIKey,

		// Brands & Workspaces (RBAC)
		"POST /v1/brands":                         s.createBrand,
		"GET /v1/brands":                          s.listBrands,
		"GET /v1/brands/{id}":                     s.getBrand,
		"PUT /v1/brands/{id}":                     s.updateBrand,
		"PATCH /v1/brands/{id}":                   s.updateBrand,
		"DELETE /v1/brands/{id}":                  s.deleteBrand,
		"POST /v1/brands/{id}/members":            s.addBrandMember,
		"GET /v1/brands/{id}/members":             s.listBrandMembers,
		"DELETE /v1/brands/{id}/members/{userId}": s.removeBrandMember,

		// Media Uploads
		"POST /v1/uploads/media": s.uploadMedia,

		// Social Handles & OAuth
		"GET /v1/social/accounts":            s.listSocialAccounts,
		"POST /v1/social/connect/{platform}": s.connectSocialPlatform,
		"GET /v1/social/connect/{platform}":  s.connectSocialPlatform,
		"POST /v1/social/refresh-token":      s.refreshSocialToken,
		"DELETE /v1/social/{id}":             s.disconnectSocialAccount,

		// Posts & Planner
		"POST /v1/posts":                        s.createPost,
		"GET /v1/posts":                         s.listPosts,
		"GET /v1/posts/{id}":                    s.getPost,
		"PUT /v1/posts/{id}":                    s.updatePost,
		"PATCH /v1/posts/{id}":                  s.updatePost,
		"DELETE /v1/posts/{id}":                 s.deletePost,
		"DELETE /v1/posts/{id}/scheduled":       s.deleteScheduledPost,
		"DELETE /v1/posts/{id}/published":       s.deletePublishedPost,
		"DELETE /v1/posts/bulk":                 s.bulkDeletePosts,
		"DELETE /v1/posts/bulk/scheduled":       s.bulkDeleteScheduledPosts,
		"DELETE /v1/posts/bulk/published":       s.bulkDeletePublishedPosts,
		"POST /v1/posts/bulk/pause-recurrence":  s.bulkPauseRecurringPosts,
		"POST /v1/posts/bulk/resume-recurrence": s.bulkResumeRecurringPosts,
		"POST /v1/posts/{id}/submit-approval":   s.submitPostForApproval,
		"POST /v1/posts/{id}/approve":           s.approvePost,
		"POST /v1/posts/{id}/reject":            s.rejectPost,
		"POST /v1/posts/{id}/reshare":           s.resharePost,
		"POST /v1/posts/{id}/publish":           s.publishPost,
		"GET /v1/posts/search/semantic":         s.semanticSearchPosts,
	}

	for pattern, h := range routes {
		if pattern == "POST /mcp" || pattern == "GET /mcp" ||
			pattern == "GET /.well-known/oauth-protected-resource" ||
			pattern == "GET /.well-known/oauth-authorization-server" ||
			pattern == "GET /oauth/authorize" ||
			pattern == "POST /oauth/authorize" ||
			pattern == "POST /oauth/token" {
			mux.Handle(pattern, h)
			continue
		}
		mux.Handle(pattern, s.authed(h))
		if strings.Contains(pattern, " /v1/") {
			aliasPattern := strings.Replace(pattern, " /v1/", " /", 1)
			mux.Handle(aliasPattern, s.authed(h))
			apiAliasPattern := strings.Replace(pattern, " /v1/", " /api/v1/", 1)
			mux.Handle(apiAliasPattern, s.authed(h))
		}
	}

	return s.recoverer(s.cors(s.securityHeaders(s.rateLimiter(s.accessLog(mux)))))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	status := "healthy"
	components := map[string]string{
		"mongodb": "healthy",
		"redis":   "healthy",
	}

	// Verify Redis readiness
	if s.redis != nil && s.redis.Client != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.redis.Client.Ping(ctx).Err(); err != nil {
			components["redis"] = "degraded"
			status = "degraded"
		}
	}

	// Verify PostgreSQL readiness (if configured)
	if s.postgres != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.postgres.Ping(ctx); err != nil {
			components["postgres"] = "degraded"
			status = "degraded"
		} else {
			components["postgres"] = "healthy"
		}
	}

	// Verify RabbitMQ readiness (if configured)
	if s.rabbitMQ != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.rabbitMQ.Health(ctx); err != nil {
			components["rabbitmq"] = "degraded"
			status = "degraded"
		} else {
			components["rabbitmq"] = "healthy"
		}
	}

	// Verify Qdrant readiness (if configured)
	if s.qdrant != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.qdrant.Health(ctx); err != nil {
			components["qdrant"] = "degraded"
			status = "degraded"
		} else {
			components["qdrant"] = "healthy"
		}
	}

	statusCode := http.StatusOK
	if r.URL.Query().Get("ready") == "1" && status == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	writeJSON(w, statusCode, map[string]any{
		"status":     status,
		"service":    "xsocial-backend",
		"time":       time.Now().UTC().Format(time.RFC3339),
		"components": components,
	})
}

// authed middleware extracts Bearer token (JWT or API Key mc_live_...), verifies scopes & permissions,
// applies per-account rate limits, and injects context variables matching mailhost-backend.
func (s *Server) authed(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)

		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		var tok string
		if authHeader != "" {
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				tok = strings.TrimSpace(authHeader[7:])
			} else {
				tok = authHeader
			}
		}
		if tok == "" {
			tok = strings.TrimSpace(r.Header.Get("X-API-Key"))
		}
		if tok == "" {
			if blocked, _ := s.checkAuthRate(ip); blocked {
				w.Header().Set("Retry-After", "300")
				writeError(w, http.StatusTooManyRequests, "too many failed authentication attempts; try again later")
				return
			}
			s.recordAuthFailure(ip)
			writeError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}

		// 1. API Key Auth Path (mc_live_... or xs_live_...)
		if strings.HasPrefix(tok, "mc_live_") || strings.HasPrefix(tok, "xs_live_") {
			if s.apiKeyService == nil {
				s.recordAuthFailure(ip)
				writeError(w, http.StatusUnauthorized, "invalid or expired API key")
				return
			}
			key, err := s.apiKeyService.ValidateApiKey(r.Context(), tok)
			if err != nil || key == nil {
				if blocked, _ := s.checkAuthRate(ip); blocked {
					w.Header().Set("Retry-After", "300")
					writeError(w, http.StatusTooManyRequests, "too many failed authentication attempts; try again later")
					return
				}
				s.recordAuthFailure(ip)
				writeError(w, http.StatusUnauthorized, "invalid or expired API key")
				return
			}
			s.recordAuthSuccess(ip)

			ctx := context.WithValue(r.Context(), userIDKey{}, key.UserID.Hex())
			ctx = context.WithValue(ctx, roleKey{}, "developer")
			ctx = context.WithValue(ctx, permKey{}, strings.Join(key.Permissions, ","))
			if key.BrandID != nil {
				ctx = context.WithValue(ctx, brandIDKey{}, key.BrandID.Hex())
			}
			h(w, r.WithContext(ctx))
			return
		}

		// 2. JWT Bearer Token Auth Path
		token, err := jwt.Parse(tok, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(s.cfg.JWTAccessSecret), nil
		})
		if err != nil || !token.Valid {
			if blocked, _ := s.checkAuthRate(ip); blocked {
				w.Header().Set("Retry-After", "300")
				writeError(w, http.StatusTooManyRequests, "too many failed authentication attempts; try again later")
				return
			}
			s.recordAuthFailure(ip)
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			if blocked, _ := s.checkAuthRate(ip); blocked {
				w.Header().Set("Retry-After", "300")
				writeError(w, http.StatusTooManyRequests, "too many failed authentication attempts; try again later")
				return
			}
			s.recordAuthFailure(ip)
			writeError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}

		sub, _ := claims["sub"].(string)
		if sub == "" {
			if blocked, _ := s.checkAuthRate(ip); blocked {
				w.Header().Set("Retry-After", "300")
				writeError(w, http.StatusTooManyRequests, "too many failed authentication attempts; try again later")
				return
			}
			s.recordAuthFailure(ip)
			writeError(w, http.StatusUnauthorized, "invalid token user")
			return
		}

		s.recordAuthSuccess(ip)
		ctx := context.WithValue(r.Context(), userIDKey{}, sub)
		ctx = context.WithValue(ctx, roleKey{}, "user")
		ctx = context.WithValue(ctx, permKey{}, "full_access")
		h(w, r.WithContext(ctx))
	})
}

// Context helpers matching mailhost-backend
func currentUserID(r *http.Request) string {
	if v, ok := r.Context().Value(userIDKey{}).(string); ok {
		return v
	}
	return ""
}

func currentUserOID(r *http.Request) (bson.ObjectID, bool) {
	uid := currentUserID(r)
	if uid == "" {
		return bson.NilObjectID, false
	}
	oid, err := bson.ObjectIDFromHex(uid)
	return oid, err == nil
}

func currentBrandID(r *http.Request) string {
	if v, ok := r.Context().Value(brandIDKey{}).(string); ok && v != "" {
		return v
	}
	if b := strings.TrimSpace(r.Header.Get("X-Brand-ID")); b != "" {
		return b
	}
	return r.URL.Query().Get("brandId")
}

func currentBrandOID(r *http.Request) (bson.ObjectID, bool) {
	bid := currentBrandID(r)
	if bid == "" {
		return bson.NilObjectID, false
	}
	oid, err := bson.ObjectIDFromHex(bid)
	return oid, err == nil
}

// requireUser extracts the authenticated user ObjectID or writes a 401 response.
func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (bson.ObjectID, bool) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return bson.NilObjectID, false
	}
	return userOID, true
}

// parsePathOID parses an ObjectID from path parameters or writes a 400 response.
func (s *Server) parsePathOID(w http.ResponseWriter, r *http.Request, param, resource string) (bson.ObjectID, bool) {
	raw := r.PathValue(param)
	oid, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid %s ID", resource))
		return bson.NilObjectID, false
	}
	return oid, true
}

// logAudit records an audit trail event for the request actor.
func (s *Server) logAudit(r *http.Request, userOID *bson.ObjectID, brandOID *bson.ObjectID, action, resType, resID string) {
	if s.auditLogService == nil {
		return
	}
	s.auditLogService.Log(r.Context(), userOID, brandOID, currentUserID(r), action, resType, resID, clientIP(r), r.UserAgent())
}

// clientIP returns the real client IP. X-Forwarded-For is used only when
// the connection comes from a loopback/private address (i.e. behind a trusted
// local proxy/load-balancer). In untrusted environments the direct RemoteAddr
// is used to prevent IP spoofing via the X-Forwarded-For header.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// Only trust X-Forwarded-For if the immediate connection is from a
	// private/loopback address (the proxy is local to us).
	if isPrivateIP(host) {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			// The leftmost non-private address is the real client IP.
			parts := strings.Split(fwd, ",")
			for _, p := range parts {
				candidate := strings.TrimSpace(p)
				if candidate != "" && !isPrivateIP(candidate) {
					return candidate
				}
			}
			// All hops are private — return the first one.
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
	}
	return host
}

// isPrivateIP reports whether ip is a loopback or RFC1918/RFC4193 private address.
func isPrivateIP(ip string) bool {
	if ip == "localhost" || ip == "::1" || ip == "127.0.0.1" {
		return true
	}
	privateRanges := []string{"10.", "172.16.", "172.17.", "172.18.", "172.19.",
		"172.20.", "172.21.", "172.22.", "172.23.", "172.24.", "172.25.", "172.26.",
		"172.27.", "172.28.", "172.29.", "172.30.", "172.31.", "192.168.", "fd", "::"}
	for _, prefix := range privateRanges {
		if strings.HasPrefix(ip, prefix) {
			return true
		}
	}
	return false
}

func (s *Server) checkAuthRate(ip string) (bool, time.Duration) {
	return s.checkAuthRateCtx(context.Background(), ip)
}

func (s *Server) checkAuthRateCtx(ctx context.Context, ip string) (bool, time.Duration) {
	if s.redis != nil {
		if ttl, err := s.redis.TTL(ctx, "auth:blocked:"+ip); err == nil && ttl > 0 {
			return true, ttl
		}
	}

	s.authMu.Lock()
	defer s.authMu.Unlock()
	rec, exists := s.authFailures[ip]
	if !exists {
		return false, 0
	}
	now := time.Now()
	if rec.blockedTo.After(now) {
		remaining := time.Until(rec.blockedTo)
		return true, remaining
	}
	return false, 0
}

func (s *Server) recordAuthFailure(ip string) {
	s.recordAuthFailureCtx(context.Background(), ip)
}

func (s *Server) recordAuthFailureCtx(ctx context.Context, ip string) {
	if s.redis != nil {
		count, err := s.redis.Incr(ctx, "auth:failed:"+ip)
		if err == nil {
			if count == 1 {
				_ = s.redis.Expire(ctx, "auth:failed:"+ip, 5*time.Minute)
			}
			if count >= 10 {
				_ = s.redis.Set(ctx, "auth:blocked:"+ip, "1", 5*time.Minute)
			}
		}
	}

	s.authMu.Lock()
	defer s.authMu.Unlock()
	if s.authFailures == nil {
		s.authFailures = make(map[string]*authFailureRecord)
	}
	now := time.Now()
	// Bound memory growth under brute-force from rotating IPs
	if len(s.authFailures) > 2000 {
		cutoff := now.Add(-10 * time.Minute)
		for k, v := range s.authFailures {
			if v.lastSeen.Before(cutoff) {
				delete(s.authFailures, k)
			}
		}
	}
	rec, exists := s.authFailures[ip]
	if !exists || now.Sub(rec.lastSeen) > 5*time.Minute {
		s.authFailures[ip] = &authFailureRecord{count: 1, lastSeen: now}
		return
	}
	rec.count++
	rec.lastSeen = now
	if rec.count >= 10 {
		rec.blockedTo = now.Add(5 * time.Minute)
	}
}

func (s *Server) recordAuthSuccess(ip string) {
	s.recordAuthSuccessCtx(context.Background(), ip)
}

func (s *Server) recordAuthSuccessCtx(ctx context.Context, ip string) {
	if s.redis != nil {
		_ = s.redis.Del(ctx, "auth:failed:"+ip, "auth:blocked:"+ip)
	}

	s.authMu.Lock()
	defer s.authMu.Unlock()
	delete(s.authFailures, ip)
}

// Middleware stack matching mailhost-backend
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimRight(strings.TrimSpace(r.Header.Get("Origin")), "/")
		allowed := false
		if s.cfg != nil {
			for _, configured := range s.cfg.ClientOrigins {
				if origin != "" && origin == strings.TrimRight(strings.TrimSpace(configured), "/") {
					allowed = true
					break
				}
			}
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID, X-API-Key, X-Brand-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			if origin != "" && !allowed {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawTraceparent := r.Header.Get("traceparent")
		reqID := r.Header.Get("X-Request-ID")

		tc, ok := utils.ParseTraceparent(rawTraceparent)
		if !ok {
			tc = utils.GenerateTraceContext(reqID)
		} else {
			if reqID == "" {
				reqID = tc.TraceID
			}
			tc.RequestID = reqID
		}

		w.Header().Set("X-Request-ID", tc.RequestID)
		w.Header().Set("traceparent", tc.Traceparent())
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		ctx := utils.ContextWithTrace(r.Context(), tc)
		ctx = context.WithValue(ctx, reqIDKey{}, tc.RequestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if s.log != nil {
			reqID := w.Header().Get("X-Request-ID")
			durMs := time.Since(start).Milliseconds()
			if rec.status >= http.StatusBadRequest || mathrand.Float64() < 0.10 {
				s.log.InfoContext(r.Context(), "http_request",
					"request_id", reqID,
					"method", r.Method,
					"path", r.URL.Path,
					"status", rec.status,
					"dur_ms", durMs,
					"ip", clientIP(r),
				)
			}
		}
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				reqID := w.Header().Get("X-Request-ID")
				stack := string(debug.Stack())
				if s.log != nil {
					s.log.ErrorContext(r.Context(), "panic_recovered", "request_id", reqID, "panic", fmt.Sprintf("%v", v), "path", r.URL.Path, "stack", stack)
				}
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) allow(ctx context.Context, key string) bool {
	limit := 500
	window := 1 * time.Second
	if s.cfg != nil && s.cfg.RateLimitMax > 0 && s.cfg.RateLimitWindowMS > 0 {
		limit = s.cfg.RateLimitMax
		window = s.cfg.RateLimitWindowMS
	}

	// 1. Distributed Stateless Rate Limiting across horizontal instances via Redis
	if s.redis != nil {
		allowed, err := s.redis.AllowRateLimit(ctx, "ratelimit:"+key, limit, window)
		if err == nil {
			return allowed
		}
	}

	// 2. In-Memory Token Bucket Fallback
	burst := limit * 2
	if burst < 10 {
		burst = 10
	}
	rateLimit := rate.Limit(float64(limit) / window.Seconds())
	v, _ := s.limiters.LoadOrStore(key, rate.NewLimiter(rateLimit, burst))
	if lim, ok := v.(*rate.Limiter); ok {
		return lim.Allow()
	}
	return true
}

func (s *Server) rateLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !s.allow(r.Context(), ip) {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// JSON & Request Helpers matching mailhost-backend
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	resp := map[string]any{
		"success":   false,
		"error":     msg,
		"message":   msg,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if reqID := w.Header().Get("X-Request-ID"); reqID != "" {
		resp["requestId"] = reqID
	}
	writeJSON(w, status, resp)
}

func writeApiError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		writeError(w, 499, "client closed request")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		writeError(w, http.StatusGatewayTimeout, "request timed out")
		return
	}
	var apiErr *utils.ApiError
	if errors.As(err, &apiErr) {
		resp := map[string]any{
			"success":   false,
			"error":     apiErr.Message,
			"message":   apiErr.Message,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		if apiErr.Details != nil {
			resp["details"] = apiErr.Details
		}
		if reqID := w.Header().Get("X-Request-ID"); reqID != "" {
			resp["requestId"] = reqID
		}
		writeJSON(w, apiErr.StatusCode, resp)
		return
	}
	writeError(w, http.StatusBadRequest, utils.SanitizeErrorMessage(err.Error()))
}

func writeSuccess(w http.ResponseWriter, status int, data any, message string) {
	resp := map[string]any{
		"success":   true,
		"data":      data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if message != "" {
		resp["message"] = message
	}
	if reqID := w.Header().Get("X-Request-ID"); reqID != "" {
		resp["requestId"] = reqID
	}
	writeJSON(w, status, resp)
}

func decode(w http.ResponseWriter, r *http.Request, limit int64, v any) bool {
	if r.Body == nil {
		writeError(w, http.StatusBadRequest, "request body cannot be empty")
		return false
	}
	if limit <= 0 {
		limit = 10 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return false
	}
	return true
}

// page parses page/limit query parameters using the shared utils.ParsePagination
// implementation which caps limit at 100 and defaults to page=1 / limit=20.
func page(r *http.Request) (pageNum, limit int) {
	p, l, _ := utils.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)
	// utils.ParsePagination defaults to limit=10; keep our API default of 20.
	if l == 10 && r.URL.Query().Get("limit") == "" {
		l = 20
	}
	return p, l
}

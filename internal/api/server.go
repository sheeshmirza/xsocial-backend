// Package api exposes the authenticated HTTP API, MCP JSON-RPC server, and public endpoints,
// following the exact routing and architectural patterns of mailhost-backend.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	mathrand "math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/time/rate"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/database"
	"xsocial-backend/internal/platform/instagram"
	"xsocial-backend/internal/platform/linkedin"
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
	cfg            *config.Config
	log            *slog.Logger
	redis          *database.RedisClient
	userRepo       *repository.UserRepository
	rfRepo         *repository.RefreshTokenRepository
	handleRepo     *repository.SocialHandleRepository
	postRepo       *repository.PostRepository
	postActRepo    *repository.PostActivityRepository
	apiKeyRepo     *repository.ApiKeyRepository
	brandRepo      *repository.BrandRepository
	inboxRepo      *repository.InboxRepository
	smartLinkRepo  *repository.SmartLinkRepository
	autolistRepo   *repository.AutolistRepository
	competitorRepo *repository.CompetitorRepository
	analyticsRepo  *repository.AnalyticsRepository
	reportRepo     *repository.ReportRepository
	webhookRepo    *repository.WebhookRepository
	auditLogRepo   *repository.AuditLogRepository

	// Services
	tokenService     *service.TokenService
	authService      *service.AuthService
	userService      *service.UserService
	aiService        *service.AIService
	uploadService    *service.UploadService
	socialService    *service.SocialService
	postPubService   *service.PostPublishService
	liActService     *service.LinkedInActivityService
	postService      *service.PostService
	apiKeyService    *service.ApiKeyService
	brandService     *service.BrandService
	inboxService     *service.InboxService
	smartLinkService *service.SmartLinkService
	autolistService  *service.AutolistService
	bestTimesService *service.BestTimesService
	competitorServ   *service.CompetitorService
	analyticsService *service.AnalyticsService
	reportService    *service.ReportService
	webhookService   *service.WebhookService
	auditLogService  *service.AuditLogService

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

func New(
	cfg *config.Config,
	log *slog.Logger,
	redis *database.RedisClient,
	userRepo *repository.UserRepository,
	rfRepo *repository.RefreshTokenRepository,
	handleRepo *repository.SocialHandleRepository,
	postRepo *repository.PostRepository,
	postActRepo *repository.PostActivityRepository,
	apiKeyRepo *repository.ApiKeyRepository,
	brandRepo *repository.BrandRepository,
	inboxRepo *repository.InboxRepository,
	smartLinkRepo *repository.SmartLinkRepository,
	autolistRepo *repository.AutolistRepository,
	competitorRepo *repository.CompetitorRepository,
	analyticsRepo *repository.AnalyticsRepository,
	reportRepo *repository.ReportRepository,
	webhookRepo *repository.WebhookRepository,
	auditLogRepo *repository.AuditLogRepository,
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
	inboxService *service.InboxService,
	smartLinkService *service.SmartLinkService,
	autolistService *service.AutolistService,
	bestTimesService *service.BestTimesService,
	competitorServ *service.CompetitorService,
	analyticsService *service.AnalyticsService,
	reportService *service.ReportService,
	webhookService *service.WebhookService,
	auditLogService *service.AuditLogService,
	igService *instagram.Service,
	liService *linkedin.Service,
) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		cfg:              cfg,
		log:              log,
		redis:            redis,
		userRepo:         userRepo,
		rfRepo:           rfRepo,
		handleRepo:       handleRepo,
		postRepo:         postRepo,
		postActRepo:      postActRepo,
		apiKeyRepo:       apiKeyRepo,
		brandRepo:        brandRepo,
		inboxRepo:        inboxRepo,
		smartLinkRepo:    smartLinkRepo,
		autolistRepo:     autolistRepo,
		competitorRepo:   competitorRepo,
		analyticsRepo:    analyticsRepo,
		reportRepo:       reportRepo,
		webhookRepo:      webhookRepo,
		auditLogRepo:     auditLogRepo,
		tokenService:     tokenService,
		authService:      authService,
		userService:      userService,
		aiService:        aiService,
		uploadService:    uploadService,
		socialService:    socialService,
		postPubService:   postPubService,
		liActService:     liActService,
		postService:      postService,
		apiKeyService:    apiKeyService,
		brandService:     brandService,
		inboxService:     inboxService,
		smartLinkService: smartLinkService,
		autolistService:  autolistService,
		bestTimesService: bestTimesService,
		competitorServ:   competitorServ,
		analyticsService: analyticsService,
		reportService:    reportService,
		webhookService:   webhookService,
		auditLogService:  auditLogService,
		igService:        igService,
		liService:        liService,
		authFailures:     make(map[string]*authFailureRecord),
	}
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

		// Public Bio Pages & Click Tracking
		"GET /v1/bio/{slug}":                   s.publicBioPage,
		"POST /v1/bio/{slug}/click/{buttonId}": s.publicTrackClick,

		// Public Web Analytics Pixel & Beacon
		"GET /v1/tracker/pixel.png": s.publicTrackerPixel,
		"POST /v1/tracker/event":    s.publicTrackerEvent,

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
		"GET /.well-known/oauth-protected-resource": s.mcpProtectedResourceMetadata,
		"GET /.well-known/oauth-authorization-server": s.mcpAuthorizationServerMetadata,
		"GET /oauth/authorize": s.mcpAuthorize,
		"POST /oauth/authorize": s.mcpAuthorize,
		"POST /oauth/token": s.mcpToken,

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

		// Best Times to Post Heatmaps
		"GET /v1/planner/best-times": s.getBestTimes,

		// Unified Inbox & Community Management
		"GET /v1/inbox":                        s.listInboxConversations,
		"GET /v1/inbox/{id}":                   s.getInboxConversation,
		"POST /v1/inbox/{id}/reply":            s.replyInboxMessage,
		"PATCH /v1/inbox/{id}/status":          s.updateInboxStatus,
		"GET /v1/inbox/canned-replies":         s.listCannedReplies,
		"POST /v1/inbox/canned-replies":        s.createCannedReply,
		"DELETE /v1/inbox/canned-replies/{id}": s.deleteCannedReply,

		// SmartLinks (Link-in-Bio)
		"POST /v1/smartlinks":               s.createSmartLink,
		"GET /v1/smartlinks":                s.listSmartLinks,
		"PUT /v1/smartlinks/{id}":           s.updateSmartLink,
		"PATCH /v1/smartlinks/{id}":         s.updateSmartLink,
		"DELETE /v1/smartlinks/{id}":        s.deleteSmartLink,
		"GET /v1/smartlinks/{id}/analytics": s.getSmartLinkAnalytics,

		// Autolists & RSS Feeds (Evergreen content queues)
		"POST /v1/autolists":            s.createAutolist,
		"GET /v1/autolists":             s.listAutolists,
		"POST /v1/autolists/{id}/posts": s.addAutolistPost,
		"DELETE /v1/autolists/{id}":     s.deleteAutolist,
		"POST /v1/rss-feeds":            s.createRSSFeed,
		"GET /v1/rss-feeds":             s.listRSSFeeds,
		"DELETE /v1/rss-feeds/{id}":     s.deleteRSSFeed,

		// Competitors
		"POST /v1/competitors":                s.addCompetitor,
		"GET /v1/competitors":                 s.listCompetitors,
		"DELETE /v1/competitors/{id}":         s.deleteCompetitor,
		"POST /v1/competitors/{id}/sync":      s.syncCompetitor,
		"POST /v1/competitors/{id}/snapshots": s.recordCompetitorSnapshot,

		// Analytics
		"GET /v1/analytics/web":             s.getWebAnalytics,
		"GET /v1/analytics/social":          s.getSocialAnalytics,
		"POST /v1/analytics/social/rollups": s.recordSocialRollup,

		// Reports
		"POST /v1/reports/generate":     s.generateReport,
		"GET /v1/reports":               s.listReports,
		"GET /v1/reports/{id}/download": s.downloadReport,

		// Webhooks
		"POST /v1/webhooks":        s.createWebhook,
		"GET /v1/webhooks":         s.listWebhooks,
		"DELETE /v1/webhooks/{id}": s.deleteWebhook,

		// Audit Logs
		"GET /v1/audit-logs": s.listAuditLogs,
	}

	for pattern, h := range routes {
		if pattern == "POST /mcp" || pattern == "GET /mcp" ||
			pattern == "GET /.well-known/oauth-protected-resource" ||
			pattern == "GET /.well-known/oauth-authorization-server" ||
			pattern == "GET /oauth/authorize" ||
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
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "healthy",
		"service": "xsocial-backend",
		"time":    time.Now().UTC().Format(time.RFC3339),
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

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (s *Server) checkAuthRate(ip string) (bool, time.Duration) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	rec, exists := s.authFailures[ip]
	if !exists {
		return false, 0
	}
	now := time.Now()
	if rec.blockedTo.After(now) {
		return true, 0
	}
	if rec.count >= 4 {
		return false, time.Second
	}
	return false, 0
}

func (s *Server) recordAuthFailure(ip string) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	if s.authFailures == nil {
		s.authFailures = make(map[string]*authFailureRecord)
	}
	now := time.Now()
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
	s.authMu.Lock()
	defer s.authMu.Unlock()
	delete(s.authFailures, ip)
}

// Middleware stack matching mailhost-backend
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID, X-API-Key, X-Brand-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		ctx := context.WithValue(r.Context(), reqIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if s.log != nil && (rec.status >= http.StatusInternalServerError || mathrand.Float64() < 0.05) {
			s.log.Info("http", "method", r.Method, "path", r.URL.Path, "status", rec.status, "dur_ms", time.Since(start).Milliseconds())
		}
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if s.log != nil {
					s.log.Error("panic", "value", v, "path", r.URL.Path)
				}
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) allow(key string) bool {
	v, _ := s.limiters.LoadOrStore(key, rate.NewLimiter(rate.Limit(500), 1000))
	return v.(*rate.Limiter).Allow()
}

func (s *Server) rateLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !s.allow(ip) {
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
	writeJSON(w, status, map[string]any{
		"success": false,
		"error":   msg,
		"message": msg,
	})
}

func writeApiError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	var apiErr *utils.ApiError
	if errors.As(err, &apiErr) {
		writeError(w, apiErr.StatusCode, apiErr.Message)
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func decode(w http.ResponseWriter, r *http.Request, limit int64, v any) bool {
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

func page(r *http.Request) (int, int) {
	page, limit := 1, 20
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	return page, limit
}

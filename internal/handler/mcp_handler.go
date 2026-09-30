package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/service"
)

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *mcpError `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string                `json:"type"`
	Properties map[string]schemaProp `json:"properties"`
	Required   []string              `json:"required,omitempty"`
}

type schemaProp struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

var metricoolMCPTools = []mcpTool{
	{
		Name:        "get_brands",
		Description: "List all managed brands and client workspaces connected to your account.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]schemaProp{},
		},
	},
	{
		Name:        "get_brand_details",
		Description: "Retrieve detailed settings, timezone, currency, and connected social networks for a specific brand.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"brand_id": {Type: "string", Description: "The hexadecimal ID of the brand."},
			},
			Required: []string{"brand_id"},
		},
	},
	{
		Name:        "create_brand",
		Description: "Create a new brand or client workspace.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"name":        {Type: "string", Description: "Name of the brand or client company."},
				"timezone":    {Type: "string", Description: "Brand timezone, e.g. 'America/New_York' or 'Europe/Madrid'."},
				"currency":    {Type: "string", Description: "Currency code, e.g. 'USD' or 'EUR'."},
				"website_url": {Type: "string", Description: "Brand website URL."},
			},
			Required: []string{"name"},
		},
	},
	{
		Name:        "get_analytics",
		Description: "Fetch performance analytics (followers, impressions, reach, engagement, clicks, video views) for a brand across social networks or web.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"brand_id": {Type: "string", Description: "The brand ID."},
				"platform": {Type: "string", Description: "Platform to query: 'instagram', 'facebook', 'linkedin', 'tiktok', 'youtube', 'twitter', 'pinterest', or 'web'."},
				"days":     {Type: "string", Description: "Number of past days to query, e.g. '30' or '7'."},
			},
			Required: []string{"brand_id", "platform"},
		},
	},
	{
		Name:        "get_best_time_to_post",
		Description: "Get the optimal hours and 7x24 heatmap to post on a given social network based on audience engagement data.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"brand_id": {Type: "string", Description: "The brand ID."},
				"platform": {Type: "string", Description: "Social network: 'instagram', 'facebook', 'linkedin', 'tiktok', or 'youtube'."},
			},
			Required: []string{"brand_id", "platform"},
		},
	},
	{
		Name:        "get_competitors",
		Description: "Retrieve competitor benchmarking data (follower count, post frequency, engagement rate) for a brand.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"brand_id": {Type: "string", Description: "The brand ID."},
			},
			Required: []string{"brand_id"},
		},
	},
	{
		Name:        "get_scheduled_posts",
		Description: "List posts by status (scheduled, draft, published, pending_approval) for a brand or account.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"status":   {Type: "string", Description: "Filter by status: 'scheduled', 'draft', 'published', 'pending_approval', or empty for all."},
				"brand_id": {Type: "string", Description: "Optional brand ID filter."},
			},
		},
	},
	{
		Name:        "create_scheduled_post",
		Description: "Create and schedule or draft a post across multiple social platforms with media, first comment, and optional review approval.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"brand_id":       {Type: "string", Description: "Brand ID to publish under."},
				"caption":        {Type: "string", Description: "Post copy / caption text."},
				"platforms":      {Type: "string", Description: "Comma-separated target platforms, e.g. 'instagram,linkedin,facebook'."},
				"scheduled_time": {Type: "string", Description: "ISO 8601 scheduled publish time (leave empty to save as draft)."},
				"media_urls":     {Type: "string", Description: "Comma-separated public URLs of images or videos to attach."},
				"first_comment":  {Type: "string", Description: "Optional auto-posted first comment (for hashtags)."},
			},
			Required: []string{"caption", "platforms"},
		},
	},
	{
		Name:        "publish_post_now",
		Description: "Publish an existing post immediately across all its selected social channels.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"post_id": {Type: "string", Description: "The ID of the post to publish."},
			},
			Required: []string{"post_id"},
		},
	},
	{
		Name:        "approve_post",
		Description: "Approve a post that is waiting in the review approval workflow.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"post_id": {Type: "string", Description: "The ID of the post to approve."},
			},
			Required: []string{"post_id"},
		},
	},
	{
		Name:        "list_inbox_messages",
		Description: "Fetch unified inbox conversation threads (DMs, comments, reviews) across social networks for a brand.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"brand_id": {Type: "string", Description: "The brand ID."},
				"platform": {Type: "string", Description: "Optional platform filter (e.g. 'instagram', 'linkedin', 'google_business')."},
				"status":   {Type: "string", Description: "Status filter: 'open' or 'resolved'."},
			},
			Required: []string{"brand_id"},
		},
	},
	{
		Name:        "reply_inbox_message",
		Description: "Post a reply to an inbox message or comment thread.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"conversation_id": {Type: "string", Description: "The ID of the conversation thread."},
				"brand_id":        {Type: "string", Description: "The brand ID."},
				"reply_text":      {Type: "string", Description: "The text message reply to send."},
			},
			Required: []string{"conversation_id", "brand_id", "reply_text"},
		},
	},
	{
		Name:        "get_smartlinks",
		Description: "Retrieve SmartLink (link-in-bio) pages and click analytics for a brand.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"brand_id": {Type: "string", Description: "The brand ID."},
			},
			Required: []string{"brand_id"},
		},
	},
	{
		Name:        "get_ad_campaigns",
		Description: "Query paid ad campaign performance across Google Ads, Meta Ads, and TikTok Ads (spend, impressions, clicks, CPC, ROAS).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"brand_id": {Type: "string", Description: "The brand ID."},
				"platform": {Type: "string", Description: "Optional ad platform: 'meta', 'google', or 'tiktok'."},
			},
			Required: []string{"brand_id"},
		},
	},
	{
		Name:        "generate_report",
		Description: "Generate a custom performance report for a brand in PDF, CSV, or JSON format.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"brand_id": {Type: "string", Description: "The brand ID."},
				"title":    {Type: "string", Description: "Report title."},
				"format":   {Type: "string", Description: "Report format: 'pdf', 'csv', or 'json'."},
			},
			Required: []string{"brand_id", "title"},
		},
	},
}

type MCPHandler struct {
	brandService     *service.BrandService
	analyticsService *service.AnalyticsService
	bestTimesService *service.BestTimesService
	competitorServ   *service.CompetitorService
	postService      *service.PostService
	publishService   *service.PostPublishService
	inboxService     *service.InboxService
	smartLinkService *service.SmartLinkService
	adService        *service.AdCampaignService
	reportService    *service.ReportService
}

func NewMCPHandler(
	brandService *service.BrandService,
	analyticsService *service.AnalyticsService,
	bestTimesService *service.BestTimesService,
	competitorServ *service.CompetitorService,
	postService *service.PostService,
	publishService *service.PostPublishService,
	inboxService *service.InboxService,
	smartLinkService *service.SmartLinkService,
	adService *service.AdCampaignService,
	reportService *service.ReportService,
) *MCPHandler {
	return &MCPHandler{
		brandService:     brandService,
		analyticsService: analyticsService,
		bestTimesService: bestTimesService,
		competitorServ:   competitorServ,
		postService:      postService,
		publishService:   publishService,
		inboxService:     inboxService,
		smartLinkService: smartLinkService,
		adService:        adService,
		reportService:    reportService,
	}
}

// HandleMCP handles Model Context Protocol requests over HTTP JSON-RPC 2.0
func (h *MCPHandler) HandleMCP(c *gin.Context) {
	if c.Request.Method == http.MethodGet {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "metricool-mcp",
			"version": "1.0.0",
			"tools":   len(metricoolMCPTools),
		})
		return
	}

	var req mcpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, mcpResponse{
			JSONRPC: "2.0",
			Error:   &mcpError{Code: -32700, Message: "Parse error"},
		})
		return
	}

	switch req.Method {
	case "initialize":
		c.JSON(http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "metricool-mcp",
					"version": "1.0.0",
				},
			},
		})

	case "notifications/initialized":
		c.Status(http.StatusNoContent)

	case "tools/list":
		c.JSON(http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": metricoolMCPTools,
			},
		})

	case "tools/call":
		var callParams struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			c.JSON(http.StatusOK, mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &mcpError{Code: -32602, Message: "Invalid params"},
			})
			return
		}

		userVal, _ := c.Get("user")
		user, _ := userVal.(*models.User)
		userID := bson.NilObjectID
		if user != nil {
			userID = user.ID
		}

		resStr, callErr := h.executeTool(c.Request.Context(), userID, callParams.Name, callParams.Arguments)
		if callErr != nil {
			c.JSON(http.StatusOK, mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"isError": true,
					"content": []map[string]string{
						{"type": "text", "text": fmt.Sprintf("Error: %v", callErr)},
					},
				},
			})
			return
		}

		c.JSON(http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]string{
					{"type": "text", "text": resStr},
				},
			},
		})

	default:
		c.JSON(http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcpError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
		})
	}
}

func (h *MCPHandler) executeTool(ctx context.Context, userID bson.ObjectID, name string, args map[string]any) (string, error) {
	getArg := func(k string) string {
		if v, ok := args[k].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}

	switch name {
	case "get_brands":
		brands, err := h.brandService.ListBrands(ctx, userID)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(brands, "", "  ")
		return string(b), nil

	case "get_brand_details":
		bid, err := bson.ObjectIDFromHex(getArg("brand_id"))
		if err != nil {
			return "", fmt.Errorf("invalid brand_id")
		}
		brand, err := h.brandService.GetBrand(ctx, bid, userID)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(brand, "", "  ")
		return string(b), nil

	case "create_brand":
		brand, err := h.brandService.CreateBrand(ctx, userID, models.CreateBrandRequest{
			Name:       getArg("name"),
			Timezone:   getArg("timezone"),
			Currency:   getArg("currency"),
			WebsiteURL: getArg("website_url"),
		})
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(brand, "", "  ")
		return string(b), nil

	case "get_analytics":
		bid, err := bson.ObjectIDFromHex(getArg("brand_id"))
		if err != nil {
			return "", fmt.Errorf("invalid brand_id")
		}
		platform := getArg("platform")
		if strings.EqualFold(platform, "web") {
			data, err := h.analyticsService.GetWebOverview(ctx, bid, 30)
			if err != nil {
				return "", err
			}
			b, _ := json.MarshalIndent(data, "", "  ")
			return string(b), nil
		}
		data, err := h.analyticsService.GetSocialOverview(ctx, bid, platform, 30)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(data, "", "  ")
		return string(b), nil

	case "get_best_time_to_post":
		bid, err := bson.ObjectIDFromHex(getArg("brand_id"))
		if err != nil {
			return "", fmt.Errorf("invalid brand_id")
		}
		platform := getArg("platform")
		data, err := h.bestTimesService.GetBestTimes(ctx, bid, platform)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(data, "", "  ")
		return string(b), nil

	case "get_competitors":
		bid, err := bson.ObjectIDFromHex(getArg("brand_id"))
		if err != nil {
			return "", fmt.Errorf("invalid brand_id")
		}
		data, err := h.competitorServ.ListCompetitors(ctx, bid)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(data, "", "  ")
		return string(b), nil

	case "get_scheduled_posts":
		queryParams := map[string]string{}
		if st := getArg("status"); st != "" {
			queryParams["status"] = st
		}
		posts, _, err := h.postService.ListPosts(ctx, userID, queryParams, 0, 50, bson.D{{Key: "createdAt", Value: -1}})
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(posts, "", "  ")
		return string(b), nil

	case "create_scheduled_post":
		platforms := strings.Split(getArg("platforms"), ",")
		for i := range platforms {
			platforms[i] = strings.TrimSpace(platforms[i])
		}

		var mediaItems []models.MediaItem
		if m := getArg("media_urls"); m != "" {
			urls := strings.Split(m, ",")
			for _, u := range urls {
				u = strings.TrimSpace(u)
				if u != "" {
					mediaItems = append(mediaItems, models.MediaItem{
						Type: "image",
						URL:  u,
					})
				}
			}
		}

		schedStr := getArg("scheduled_time")
		var schedTime *time.Time
		status := models.PostStatusDraft
		if schedStr != "" {
			t, err := time.Parse(time.RFC3339, schedStr)
			if err == nil {
				schedTime = &t
				status = models.PostStatusScheduled
			}
		}

		var brandOID *bson.ObjectID
		if bid := getArg("brand_id"); bid != "" {
			if b, err := bson.ObjectIDFromHex(bid); err == nil {
				brandOID = &b
			}
		}

		post, err := h.postService.CreatePost(ctx, userID, models.CreatePostRequest{
			Caption:           getArg("caption"),
			SelectedPlatforms: platforms,
			ScheduledTime:     schedTime,
			Status:            status,
			Media:             mediaItems,
		})
		if err != nil {
			return "", err
		}
		if brandOID != nil || getArg("first_comment") != "" {
			post.BrandID = brandOID
			post.FirstComment = getArg("first_comment")
			_, _ = h.postService.UpdatePost(ctx, userID, post.ID, models.UpdatePostRequest{
				FirstComment: getArg("first_comment"),
				BrandID:      brandOID,
			})
		}

		b, _ := json.MarshalIndent(post, "", "  ")
		return string(b), nil

	case "publish_post_now":
		pid, err := bson.ObjectIDFromHex(getArg("post_id"))
		if err != nil {
			return "", fmt.Errorf("invalid post_id")
		}
		post, err := h.postService.GetPost(ctx, userID, pid)
		if err != nil {
			return "", err
		}
		responses, err := h.publishService.PublishToPlatforms(ctx, userID, post, post.SelectedPlatforms, post.SelectedAccountIds)
		if err != nil {
			return "", err
		}
		pubStatus := models.PostStatusPublished
		now := time.Now()
		post.Status = pubStatus
		post.PublishedTime = &now
		post.PlatformResponses = responses
		_, _ = h.postService.UpdatePost(ctx, userID, post.ID, models.UpdatePostRequest{
			Status: &pubStatus,
		})
		b, _ := json.MarshalIndent(post, "", "  ")
		return string(b), nil

	case "approve_post":
		pid, err := bson.ObjectIDFromHex(getArg("post_id"))
		if err != nil {
			return "", fmt.Errorf("invalid post_id")
		}
		schedStatus := models.PostStatusScheduled
		post, err := h.postService.UpdatePost(ctx, userID, pid, models.UpdatePostRequest{
			Status:       &schedStatus,
			ReviewStatus: "approved",
		})
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(post, "", "  ")
		return string(b), nil

	case "list_inbox_messages":
		bid, err := bson.ObjectIDFromHex(getArg("brand_id"))
		if err != nil {
			return "", fmt.Errorf("invalid brand_id")
		}
		convs, total, err := h.inboxService.ListConversations(ctx, bid, userID, getArg("status"), getArg("platform"), 1, 20)
		if err != nil {
			return "", err
		}
		resp := map[string]any{"total": total, "conversations": convs}
		b, _ := json.MarshalIndent(resp, "", "  ")
		return string(b), nil

	case "reply_inbox_message":
		cid, err := bson.ObjectIDFromHex(getArg("conversation_id"))
		if err != nil {
			return "", fmt.Errorf("invalid conversation_id")
		}
		bid, err := bson.ObjectIDFromHex(getArg("brand_id"))
		if err != nil {
			return "", fmt.Errorf("invalid brand_id")
		}
		msg, err := h.inboxService.ReplyToConversation(ctx, cid, bid, userID, "AI Assistant", models.ReplyInboxRequest{
			MessageText: getArg("reply_text"),
		})
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(msg, "", "  ")
		return string(b), nil

	case "get_smartlinks":
		bid, err := bson.ObjectIDFromHex(getArg("brand_id"))
		if err != nil {
			return "", fmt.Errorf("invalid brand_id")
		}
		pages, err := h.smartLinkService.ListPagesByBrand(ctx, bid)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(pages, "", "  ")
		return string(b), nil

	case "get_ad_campaigns":
		bid, err := bson.ObjectIDFromHex(getArg("brand_id"))
		if err != nil {
			return "", fmt.Errorf("invalid brand_id")
		}
		camps, err := h.adService.ListCampaigns(ctx, bid, getArg("platform"))
		if err != nil {
			return "", err
		}
		summary, _ := h.adService.GetSummary(ctx, bid)
		resp := map[string]any{"campaigns": camps, "summary": summary}
		b, _ := json.MarshalIndent(resp, "", "  ")
		return string(b), nil

	case "generate_report":
		bid := getArg("brand_id")
		rep, err := h.reportService.GenerateReport(ctx, userID, models.GenerateReportRequest{
			BrandID: bid,
			Title:   getArg("title"),
			Format:  getArg("format"),
		})
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(rep, "", "  ")
		return string(b), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

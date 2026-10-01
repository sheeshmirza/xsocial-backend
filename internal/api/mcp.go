package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
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
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	InputSchema     inputSchema    `json:"inputSchema"`
	SecuritySchemes []any          `json:"securitySchemes,omitempty"`
	Meta            map[string]any `json:"_meta,omitempty"`
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
		Name: "get_profile",
		Description: "Return the xSocial account represented by the current authenticated connection.",
		InputSchema: inputSchema{Type:"object", Properties:map[string]schemaProp{}},
		SecuritySchemes: []any{map[string]any{"type":"oauth2","scopes":[]string{mcpOAuthScope}}},
		Meta: map[string]any{"openai/profile":true},
	},
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

func initMCPToolSecurity() {
	for i := range metricoolMCPTools {
		if len(metricoolMCPTools[i].SecuritySchemes) == 0 {
			metricoolMCPTools[i].SecuritySchemes = []any{map[string]any{"type":"oauth2","scopes":[]string{mcpOAuthScope}}}
		}
		if metricoolMCPTools[i].Meta == nil {
			metricoolMCPTools[i].Meta = map[string]any{}
		}
		metricoolMCPTools[i].Meta["securitySchemes"] = metricoolMCPTools[i].SecuritySchemes
	}
}

// handleMCP processes JSON-RPC 2.0 requests conforming to the Model Context Protocol (MCP) exactly like mailhost-backend
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	initMCPToolSecurity()
	// MCP Streamable HTTP is POST-based. This implementation intentionally runs
	// statelessly: each JSON-RPC exchange is self-contained and returns JSON.
	if r.Method == http.MethodGet {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	var req mcpRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, mcpResponse{
			JSONRPC: "2.0",
			Error:   &mcpError{Code: -32700, Message: "Parse error"},
		})
		return
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, mcpResponse{
			JSONRPC: "2.0",
			Error:   &mcpError{Code: -32600, Message: "Invalid Request"},
		})
		return
	}

	switch req.Method {
	case "initialize":
		requestedVersion := r.Header.Get("MCP-Protocol-Version")
		protocolVersion := "2025-06-18"
		if requestedVersion == "2024-11-05" {
			protocolVersion = requestedVersion
		}
		w.Header().Set("MCP-Protocol-Version", protocolVersion)
		writeJSON(w, http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": protocolVersion,
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "xsocial-mcp",
					"version": "1.0.0",
				},
			},
		})

	case "notifications/initialized":
		w.WriteHeader(http.StatusNoContent)

	case "tools/list":
		writeJSON(w, http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": metricoolMCPTools,
			},
		})

	case "tools/call":
		userOID, ok := s.authenticateMCPBearer(r)
		if !ok {
			s.mcpAuthChallenge(w, r)
			return
		}
		var callParams struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			writeJSON(w, http.StatusOK, mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &mcpError{Code: -32602, Message: "Invalid params"},
			})
			return
		}

		resStr, callErr := s.executeMCPTool(r.Context(), userOID, callParams.Name, callParams.Arguments)
		if callErr != nil {
			writeJSON(w, http.StatusOK, mcpResponse{
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

		writeJSON(w, http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]string{
					{"type": "text", "text": resStr},
				},
			},
		})

	default:
		writeJSON(w, http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcpError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
		})
	}
}

func (s *Server) executeMCPTool(ctx context.Context, userID bson.ObjectID, name string, args map[string]any) (string, error) {
	getArg := func(k string) string {
		if v, ok := args[k].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}

	switch name {
	case "get_profile":
		user, err := s.userService.GetUserByID(ctx, userID)
		if err != nil || user == nil {
			return "", fmt.Errorf("authenticated user not found")
		}
		return mcpOAuthJSON(map[string]any{"id":user.ID.Hex(),"name":user.FullName,"email":user.Email,"nickname":"xSocial"}), nil

	case "get_brands":
		brands, err := s.brandService.ListBrands(ctx, userID)
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
		brand, err := s.brandService.GetBrand(ctx, bid, userID)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(brand, "", "  ")
		return string(b), nil

	case "create_brand":
		brand, err := s.brandService.CreateBrand(ctx, userID, models.CreateBrandRequest{
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
			data, err := s.analyticsService.GetWebOverview(ctx, bid, 30)
			if err != nil {
				return "", err
			}
			b, _ := json.MarshalIndent(data, "", "  ")
			return string(b), nil
		}
		data, err := s.analyticsService.GetSocialOverview(ctx, bid, platform, 30)
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
		data, err := s.bestTimesService.GetBestTimes(ctx, bid, platform)
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
		data, err := s.competitorServ.ListCompetitors(ctx, bid)
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
		posts, _, err := s.postService.ListPosts(ctx, userID, queryParams, 0, 50, bson.D{{Key: "createdAt", Value: -1}})
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

		post, err := s.postService.CreatePost(ctx, userID, models.CreatePostRequest{
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
			_, _ = s.postService.UpdatePost(ctx, userID, post.ID, models.UpdatePostRequest{
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
		post, err := s.postService.GetPost(ctx, userID, pid)
		if err != nil {
			return "", err
		}
		responses, err := s.postPubService.PublishToPlatforms(ctx, userID, post, post.SelectedPlatforms, post.SelectedAccountIds)
		if err != nil {
			return "", err
		}
		pubStatus := models.PostStatusPublished
		now := time.Now()
		post.Status = pubStatus
		post.PublishedTime = &now
		post.PlatformResponses = responses
		_, _ = s.postService.UpdatePost(ctx, userID, post.ID, models.UpdatePostRequest{
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
		post, err := s.postService.UpdatePost(ctx, userID, pid, models.UpdatePostRequest{
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
		convs, total, err := s.inboxService.ListConversations(ctx, bid, userID, getArg("status"), getArg("platform"), 1, 20)
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
		msg, err := s.inboxService.ReplyToConversation(ctx, cid, bid, userID, "AI Assistant", models.ReplyInboxRequest{
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
		pages, err := s.smartLinkService.ListPagesByBrand(ctx, bid)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(pages, "", "  ")
		return string(b), nil

	case "generate_report":
		bid := getArg("brand_id")
		rep, err := s.reportService.GenerateReport(ctx, userID, models.GenerateReportRequest{
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

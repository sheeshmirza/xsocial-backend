package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	OutputSchema    map[string]any `json:"outputSchema,omitempty"`
	SecuritySchemes []any          `json:"securitySchemes,omitempty"`
	Annotations     map[string]any `json:"annotations,omitempty"`
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
		Name:        "get_profile",
		Description: "Return the xSocial account represented by the current authenticated connection.",
		InputSchema: inputSchema{Type: "object", Properties: map[string]schemaProp{}},
		OutputSchema: map[string]any{
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "minLength": 1, "pattern": "\\S"},
				"name": map[string]any{"type": "string"},
				"email": map[string]any{"type": "string"},
				"nickname": map[string]any{"type": "string"},
			},
			"required": []string{"id"},
			"additionalProperties": false,
		},
		Annotations:     map[string]any{"readOnlyHint": true, "destructiveHint": false, "openWorldHint": false},
		SecuritySchemes: []any{map[string]any{"type": "oauth2", "scopes": []string{mcpOAuthScope}}},
		Meta:            map[string]any{"openai/profile": true},
	},
	{
		Name:        "get_social_accounts",
		Description: "List connected social accounts safely (platform, username, display name, status, expiry) without exposing provider access or refresh tokens.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"platform": {Type: "string", Description: "Optional platform filter, e.g. 'instagram' or 'linkedin'."},
			},
		},
	},
	{
		Name:        "get_analytics_available_metrics",
		Description: "List the analytics metrics supported by xSocial for a social platform.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"platform": {Type: "string", Description: "Optional platform filter."},
			},
		},
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
		Name:        "get_analytics_data",
		Description: "Retrieve dated analytics rollups for a brand and optionally select specific metrics, similar to a social analytics dashboard.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"brand_id": {Type: "string", Description: "The brand ID."},
				"platform": {Type: "string", Description: "Social network, e.g. 'instagram' or 'linkedin'."},
				"metrics": {Type: "string", Description: "Optional comma-separated metrics such as followers,impressions,reach,engagement,clicks,video_views,posts_count."},
				"days": {Type: "string", Description: "Number of trailing days to return, e.g. '7' or '30'."},
			},
			Required: []string{"brand_id", "platform"},
		},
	},
	{
		Name:        "update_scheduled_post",
		Description: "Update a user's scheduled post content, media, platforms, or scheduled time.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"post_id": {Type: "string", Description: "The post ID."},
				"caption": {Type: "string", Description: "Optional new caption."},
				"platforms": {Type: "string", Description: "Optional comma-separated platforms."},
				"media_urls": {Type: "string", Description: "Optional comma-separated public media URLs."},
				"scheduled_time": {Type: "string", Description: "Optional new ISO 8601 scheduled publish time."},
				"first_comment": {Type: "string", Description: "Optional first comment."},
			},
			Required: []string{"post_id"},
		},
	},
	{
		Name:        "create_scheduled_post_for_review",
		Description: "Create a new scheduled or draft post and place it into the review/approval workflow.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"brand_id": {Type: "string", Description: "Optional brand ID."},
				"caption": {Type: "string", Description: "Post caption."},
				"platforms": {Type: "string", Description: "Comma-separated target platforms."},
				"scheduled_time": {Type: "string", Description: "Optional ISO 8601 scheduled publish time."},
				"media_urls": {Type: "string", Description: "Optional comma-separated public media URLs."},
				"first_comment": {Type: "string", Description: "Optional first comment."},
			},
			Required: []string{"caption", "platforms"},
		},
	},
	{
		Name:        "send_scheduled_post_for_review",
		Description: "Send an existing scheduled or draft post into the review/approval workflow.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]schemaProp{
				"post_id": {Type: "string", Description: "The post ID."},
			},
			Required: []string{"post_id"},
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
	readOnly := map[string]bool{
		"get_profile": true,
		"get_brands": true,
		"get_brand_details": true,
		"get_analytics": true,
		"get_best_time_to_post": true,
		"get_competitors": true,
		"get_scheduled_posts": true,
		"list_inbox_messages": true,
		"get_smartlinks": true,
		"get_social_accounts": true,
		"get_analytics_available_metrics": true,
		"get_analytics_data": true,
	}
	for i := range metricoolMCPTools {
		if len(metricoolMCPTools[i].SecuritySchemes) == 0 {
			metricoolMCPTools[i].SecuritySchemes = []any{map[string]any{"type": "oauth2", "scopes": []string{mcpOAuthScope}}}
		}
		metricoolMCPTools[i].Annotations = map[string]any{
			"readOnlyHint":    readOnly[metricoolMCPTools[i].Name],
			"destructiveHint": false,
			"openWorldHint":   false,
		}
		if metricoolMCPTools[i].Name == "publish_post_now" || metricoolMCPTools[i].Name == "reply_inbox_message" {
			metricoolMCPTools[i].Annotations["destructiveHint"] = true
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
				"instructions": "Use get_profile to identify the connected xSocial account. Use get_social_accounts and get_brands to understand the workspace before publishing. Treat brand IDs, account IDs, and post IDs as untrusted identifiers and rely on server-side authorization for every operation. Use read-only inspection tools before consequential write actions.",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "xsocial-mcp",
					"version": "1.2.0",
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
			s.mcpAuthChallenge(w, r, req.ID)
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

		result := map[string]any{
			"content": []map[string]string{
				{"type": "text", "text": resStr},
			},
		}
		if callParams.Name == "get_profile" {
			var profile map[string]any
			if err := json.Unmarshal([]byte(resStr), &profile); err != nil || strings.TrimSpace(fmt.Sprint(profile["id"])) == "" {
				writeJSON(w, http.StatusOK, mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]any{
						"isError": true,
						"content": []map[string]string{{"type": "text", "text": "Profile identity unavailable."}},
					},
				})
				return
			}
			result["structuredContent"] = profile
		}
		writeJSON(w, http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: result,
		})

	default:
		writeJSON(w, http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcpError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
		})
	}
}

func (s *Server) ensureMCPBrandAccess(ctx context.Context, userID, brandID bson.ObjectID) error {
	if brandID.IsZero() {
		return fmt.Errorf("brand_id is required")
	}
	if _, err := s.brandService.GetBrand(ctx, brandID, userID); err != nil {
		return fmt.Errorf("brand access denied: %w", err)
	}
	return nil
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

	case "get_social_accounts":
		accounts, _, err := s.socialService.ListAccounts(ctx, userID, getArg("platform"), 0, 100, bson.D{{Key: "updatedAt", Value: -1}})
		if err != nil {
			return "", err
		}
		safe := make([]map[string]any, 0, len(accounts))
		for _, account := range accounts {
			status := "connected"
			if account.IsExpired() {
				status = "expired"
			}
			safe = append(safe, map[string]any{
				"id": account.ID.Hex(),
				"brand_id": func() string { if account.BrandID == nil { return "" }; return account.BrandID.Hex() }(),
				"platform": account.Platform,
				"platform_id": account.PlatformID,
				"username": account.PlatformUsername,
				"display_name": account.PlatformDisplayName,
				"status": status,
				"access_token_expires_at": account.PlatformAccessTokenExpiry,
				"refresh_token_expires_at": account.PlatformRefreshTokenExpiry,
				"metadata": account.PlatformMetadata,
			})
		}
		return mcpOAuthJSON(safe), nil

	case "get_analytics_available_metrics":
		metrics := []map[string]string{
			{"id":"followers","label":"Followers"},
			{"id":"followers_gained","label":"Followers gained"},
			{"id":"impressions","label":"Impressions"},
			{"id":"reach","label":"Reach"},
			{"id":"engagement","label":"Engagement"},
			{"id":"clicks","label":"Clicks"},
			{"id":"video_views","label":"Video views"},
			{"id":"posts_count","label":"Posts"},
			{"id":"interactions_count","label":"Interactions"},
		}
		return mcpOAuthJSON(map[string]any{"platform": getArg("platform"), "metrics": metrics}), nil

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
		if err := s.ensureMCPBrandAccess(ctx, userID, bid); err != nil {
			return "", err
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
		if err := s.ensureMCPBrandAccess(ctx, userID, bid); err != nil {
			return "", err
		}
		platform := getArg("platform")
		days := 30
		if rawDays := getArg("days"); rawDays != "" {
			if parsed, err := strconv.Atoi(rawDays); err == nil && parsed > 0 && parsed <= 365 {
				days = parsed
			}
		}
		if strings.EqualFold(platform, "web") {
			data, err := s.analyticsService.GetWebOverview(ctx, bid, days)
			if err != nil {
				return "", err
			}
			b, _ := json.MarshalIndent(data, "", "  ")
			return string(b), nil
		}
		data, err := s.analyticsService.GetSocialOverview(ctx, bid, platform, days)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(data, "", "  ")
		return string(b), nil

	case "get_analytics_data":
		bid, err := bson.ObjectIDFromHex(getArg("brand_id"))
		if err != nil {
			return "", fmt.Errorf("invalid brand_id")
		}
		if err := s.ensureMCPBrandAccess(ctx, userID, bid); err != nil {
			return "", err
		}
		platform := getArg("platform")
		days := 30
		if rawDays := getArg("days"); rawDays != "" {
			if parsed, err := strconv.Atoi(rawDays); err == nil && parsed > 0 && parsed <= 365 {
				days = parsed
			}
		}
		rollups, err := s.analyticsService.GetSocialOverview(ctx, bid, platform, days)
		if err != nil {
			return "", err
		}
		wanted := map[string]bool{}
		if raw := getArg("metrics"); raw != "" {
			for _, m := range strings.Split(raw, ",") {
				wanted[strings.TrimSpace(strings.ToLower(m))] = true
			}
		}
		rows := make([]map[string]any, 0, len(rollups))
		for _, r := range rollups {
			row := map[string]any{"date": r.Date, "platform": r.Platform}
			add := func(name string, value any) { if len(wanted) == 0 || wanted[name] { row[name] = value } }
			add("followers", r.Followers)
			add("followers_gained", r.FollowersGained)
			add("impressions", r.Impressions)
			add("reach", r.Reach)
			add("engagement", r.Engagement)
			add("clicks", r.Clicks)
			add("video_views", r.VideoViews)
			add("posts_count", r.PostsCount)
			add("interactions_count", r.InteractionsCount)
			rows = append(rows, row)
		}
		return mcpOAuthJSON(map[string]any{"brand_id": bid.Hex(), "platform": platform, "days": days, "data": rows}), nil

	case "get_best_time_to_post":
		bid, err := bson.ObjectIDFromHex(getArg("brand_id"))
		if err != nil {
			return "", fmt.Errorf("invalid brand_id")
		}
		if err := s.ensureMCPBrandAccess(ctx, userID, bid); err != nil {
			return "", err
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
		if err := s.ensureMCPBrandAccess(ctx, userID, bid); err != nil {
			return "", err
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
		if brandID := getArg("brand_id"); brandID != "" {
			bid, err := bson.ObjectIDFromHex(brandID)
			if err != nil {
				return "", fmt.Errorf("invalid brand_id")
			}
			if err := s.ensureMCPBrandAccess(ctx, userID, bid); err != nil {
				return "", err
			}
			queryParams["brandId"] = brandID
		}
		posts, _, err := s.postService.ListPosts(ctx, userID, queryParams, 0, 50, bson.D{{Key: "createdAt", Value: -1}})
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(posts, "", "  ")
		return string(b), nil

	case "create_scheduled_post":
		if bid := getArg("brand_id"); bid != "" {
			brandOID, err := bson.ObjectIDFromHex(bid)
			if err != nil {
				return "", fmt.Errorf("invalid brand_id")
			}
			if err := s.ensureMCPBrandAccess(ctx, userID, brandOID); err != nil {
				return "", err
			}
		}
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
		if err := s.ensureMCPBrandAccess(ctx, userID, bid); err != nil {
			return "", err
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
		if err := s.ensureMCPBrandAccess(ctx, userID, bid); err != nil {
			return "", err
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
		if err := s.ensureMCPBrandAccess(ctx, userID, bid); err != nil {
			return "", err
		}
		pages, err := s.smartLinkService.ListPagesByBrand(ctx, bid)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(pages, "", "  ")
		return string(b), nil

	case "update_scheduled_post":
		pid, err := bson.ObjectIDFromHex(getArg("post_id"))
		if err != nil { return "", fmt.Errorf("invalid post_id") }
		post, err := s.postService.GetPost(ctx, userID, pid)
		if err != nil { return "", err }
		if post.BrandID != nil && !post.BrandID.IsZero() {
			if err := s.ensureMCPBrandAccess(ctx, userID, *post.BrandID); err != nil { return "", err }
		}
		update := models.UpdatePostInput{}
		if v := getArg("caption"); v != "" { update.Caption = &v }
		if v := getArg("platforms"); v != "" {
			parts := strings.Split(v, ","); for i := range parts { parts[i] = strings.TrimSpace(parts[i]) }
			update.SelectedPlatforms = &parts
		}
		if v := getArg("media_urls"); v != "" {
			urls := strings.Split(v, ","); items := make([]models.MediaItem,0,len(urls))
			for _, u := range urls { u=strings.TrimSpace(u); if u!="" { items=append(items, models.MediaItem{Type:"image",URL:u}) } }
			update.Media = &items
		}
		if v := getArg("first_comment"); v != "" { update.FirstComment = v }
		updated, err := s.postService.UpdatePost(ctx, userID, pid, update)
		if err != nil { return "", err }
		if raw := getArg("scheduled_time"); raw != "" {
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil { return "", fmt.Errorf("scheduled_time must be ISO 8601/RFC3339") }
			updated, err = s.postService.SchedulePost(ctx, userID, pid, models.SchedulePostInput{ScheduledTime:&t, Timezone:updated.Timezone, Recurrence:updated.Recurrence})
			if err != nil { return "", err }
		}
		b, _ := json.MarshalIndent(updated, "", "  "); return string(b), nil

	case "create_scheduled_post_for_review":
		if bid := getArg("brand_id"); bid != "" {
			boid, err := bson.ObjectIDFromHex(bid); if err != nil { return "", fmt.Errorf("invalid brand_id") }
			if err := s.ensureMCPBrandAccess(ctx,userID,boid); err != nil { return "", err }
		}
		platforms := strings.Split(getArg("platforms"), ","); for i:=range platforms { platforms[i]=strings.TrimSpace(platforms[i]) }
		mediaItems := []models.MediaItem{}
		if raw := getArg("media_urls"); raw != "" {
			for _, u := range strings.Split(raw,",") { u=strings.TrimSpace(u); if u!="" { mediaItems=append(mediaItems,models.MediaItem{Type:"image",URL:u}) } }
		}
		var sched *time.Time
		status := models.PostStatusDraft
		if raw := getArg("scheduled_time"); raw != "" {
			t, err := time.Parse(time.RFC3339,raw); if err != nil { return "", fmt.Errorf("scheduled_time must be ISO 8601/RFC3339") }
			sched=&t; status=models.PostStatusScheduled
		}
		var brandOID *bson.ObjectID
		if bid := getArg("brand_id"); bid != "" { if b, err := bson.ObjectIDFromHex(bid); err == nil { brandOID=&b } }
		post, err := s.postService.CreatePost(ctx,userID,models.CreatePostRequest{
			Caption:getArg("caption"), SelectedPlatforms:platforms, ScheduledTime:sched, Status:status,
			Media:mediaItems, BrandID:brandOID, FirstComment:getArg("first_comment"),
		})
		if err != nil { return "", err }
		post, err = s.postService.SubmitForApproval(ctx,userID,post.ID)
		if err != nil { return "", err }
		b,_:=json.MarshalIndent(post,"","  "); return string(b),nil

	case "send_scheduled_post_for_review":
		pid, err := bson.ObjectIDFromHex(getArg("post_id")); if err != nil { return "", fmt.Errorf("invalid post_id") }
		post, err := s.postService.GetPost(ctx,userID,pid); if err != nil { return "", err }
		if post.BrandID != nil && !post.BrandID.IsZero() { if err := s.ensureMCPBrandAccess(ctx,userID,*post.BrandID); err != nil { return "", err } }
		post, err = s.postService.SubmitForApproval(ctx,userID,pid); if err != nil { return "", err }
		b,_:=json.MarshalIndent(post,"","  "); return string(b),nil

	case "generate_report":
		bid := getArg("brand_id")
		brandOID, err := bson.ObjectIDFromHex(bid)
		if err != nil {
			return "", fmt.Errorf("invalid brand_id")
		}
		if err := s.ensureMCPBrandAccess(ctx, userID, brandOID); err != nil {
			return "", err
		}
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

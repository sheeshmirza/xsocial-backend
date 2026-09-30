package linkedin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/models"
	"xsocial-backend/internal/platform/common"
)

type ActivityAdapter struct {
	version string
	http    *common.HTTPClient
}

func NewActivityAdapter(cfg config.OAuthPlatformConfig) *ActivityAdapter {
	ver := cfg.Version
	if ver == "" {
		ver = "202607"
	}
	return &ActivityAdapter{
		version: ver,
		http:    common.NewHTTPClient(30 * time.Second),
	}
}

func (a *ActivityAdapter) NormalizePostURN(platformPostID string) string {
	val := strings.TrimSpace(platformPostID)
	if val == "" {
		return ""
	}
	if strings.HasPrefix(val, "urn:li:") {
		return val
	}
	if strings.HasPrefix(val, "ugcPost:") || strings.HasPrefix(val, "share:") {
		return fmt.Sprintf("urn:li:%s", val)
	}
	return fmt.Sprintf("urn:li:ugcPost:%s", val)
}

func (a *ActivityAdapter) PostURNToPlatformPostID(postURN string) string {
	val := strings.TrimSpace(postURN)
	if strings.HasPrefix(val, "urn:li:") {
		return strings.TrimPrefix(val, "urn:li:")
	}
	return val
}

func (a *ActivityAdapter) ExtractPostText(post HistoricalPost) string {
	if post.Commentary != nil {
		if s, ok := post.Commentary.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
		if m, ok := post.Commentary.(map[string]any); ok {
			if t, ok := m["text"].(string); ok && strings.TrimSpace(t) != "" {
				return strings.TrimSpace(t)
			}
		}
	}

	if post.SpecificContent != nil {
		if shareContent, ok := post.SpecificContent["com.linkedin.ugc.ShareContent"].(map[string]any); ok {
			if comm, ok := shareContent["shareCommentary"].(map[string]any); ok {
				if t, ok := comm["text"].(string); ok {
					return strings.TrimSpace(t)
				}
			}
		}
	}

	return ""
}

func (a *ActivityAdapter) getAuthorTypeAndURN(handle *models.SocialHandle) (authorType, authorURN string) {
	accountType := ""
	if handle.PlatformMetadata != nil {
		if at, ok := handle.PlatformMetadata["accountType"].(string); ok {
			accountType = at
		}
	}

	if models.IsLinkedInOrganization(accountType) {
		if orgURN, ok := handle.PlatformMetadata["organizationUrn"].(string); ok && orgURN != "" {
			return "organization", orgURN
		}
		return "organization", fmt.Sprintf("urn:li:organization:%s", handle.PlatformID)
	}

	return "person", fmt.Sprintf("urn:li:person:%s", handle.PlatformID)
}

func (a *ActivityAdapter) getGrantedScopes(handle *models.SocialHandle) []string {
	if handle.PlatformMetadata == nil {
		return []string{}
	}

	rawScopes := handle.PlatformMetadata["scopes"]
	if rawScopes == nil {
		rawScopes = handle.PlatformMetadata["scope"]
	}
	if rawScopes == nil {
		rawScopes = handle.PlatformMetadata["grantedScopes"]
	}

	if rawScopes == nil {
		return []string{}
	}

	if arr, ok := rawScopes.([]any); ok {
		res := make([]string, 0, len(arr))
		for _, s := range arr {
			res = append(res, fmt.Sprintf("%v", s))
		}
		return res
	}

	if str, ok := rawScopes.(string); ok {
		return strings.Fields(str)
	}

	return []string{}
}

func (a *ActivityAdapter) FetchAuthorHistoricalPosts(ctx context.Context, accessToken string, handle *models.SocialHandle, maxPosts int) ([]HistoricalPost, error) {
	token := common.NormalizeToken(accessToken)
	if token == "" {
		return nil, fmt.Errorf("LinkedIn access token is required")
	}

	authorType, authorURN := a.getAuthorTypeAndURN(handle)
	grantedScopes := a.getGrantedScopes(handle)

	// Check scopes to avoid 403 errors:
	// Person requires r_member_social
	// Organization requires r_organization_social
	if authorType == "person" && len(grantedScopes) > 0 {
		hasScope := false
		for _, s := range grantedScopes {
			if s == "r_member_social" {
				hasScope = true
				break
			}
		}
		if !hasScope {
			log.Printf("[LinkedInActivity] r_member_social not granted for %s, skipping historical posts", authorURN)
			return []HistoricalPost{}, nil
		}
	} else if authorType == "organization" && len(grantedScopes) > 0 {
		hasScope := false
		for _, s := range grantedScopes {
			if s == "r_organization_social" {
				hasScope = true
				break
			}
		}
		if !hasScope {
			log.Printf("[LinkedInActivity] r_organization_social not granted for %s, skipping historical posts", authorURN)
			return []HistoricalPost{}, nil
		}
	}

	targetCount := maxPosts
	if targetCount <= 0 {
		targetCount = 100
	}

	results := make([]HistoricalPost, 0)
	start := 0
	pageSize := 100

	headers := map[string]string{
		"Authorization":             fmt.Sprintf("Bearer %s", token),
		"Linkedin-Version":          a.version,
		"X-RestLi-Protocol-Version": "2.0.0",
	}

	for len(results) < targetCount {
		count := targetCount - len(results)
		if count > pageSize {
			count = pageSize
		}

		params := url.Values{}
		params.Set("q", "author")
		params.Set("author", authorURN)
		params.Set("count", strconv.Itoa(count))
		params.Set("start", strconv.Itoa(start))
		params.Set("sortBy", "LAST_MODIFIED")

		reqURL := fmt.Sprintf("%s/rest/posts", LinkedInAPIBase)
		body, status, err := a.http.Get(ctx, reqURL, params, headers)
		if err != nil || status >= 400 {
			break
		}

		var resp HistoricalPostResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			break
		}

		if len(resp.Elements) == 0 {
			break
		}

		results = append(results, resp.Elements...)
		if len(resp.Elements) < count || (resp.Paging.Total > 0 && start+count >= resp.Paging.Total) {
			break
		}
		start += count
	}

	return results, nil
}

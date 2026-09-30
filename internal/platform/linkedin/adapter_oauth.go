package linkedin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/platform/common"
	"xsocial-backend/internal/utils"
)

var linkedInOAuthScopes = []string{
	"email",
	"openid",
	"profile",
	"r_1st_connections_size",
	"r_basicprofile",
	"r_member_profileAnalytics",
	"r_member_social",
	"r_organization_admin",
	"r_organization_social",
	"rw_organization_admin",
	"w_member_social",
	"w_organization_social",
}

type OAuthAdapter struct {
	config config.OAuthPlatformConfig
	http   *common.HTTPClient
}

func NewOAuthAdapter(cfg config.OAuthPlatformConfig) *OAuthAdapter {
	return &OAuthAdapter{
		config: cfg,
		http:   common.NewHTTPClient(30 * time.Second),
	}
}

func (a *OAuthAdapter) BuildAuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", a.config.ClientID)
	q.Set("redirect_uri", a.config.RedirectURI)
	q.Set("state", state)
	q.Set("scope", strings.Join(linkedInOAuthScopes, " "))

	return fmt.Sprintf("https://www.linkedin.com/oauth/v2/authorization?%s", q.Encode())
}

func (a *OAuthAdapter) ExchangeCodeForToken(ctx context.Context, code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", a.config.RedirectURI)
	data.Set("client_id", a.config.ClientID)
	data.Set("client_secret", a.config.ClientSecret)

	body, status, err := a.http.PostForm(ctx, "https://www.linkedin.com/oauth/v2/accessToken", data, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, utils.BadRequest(fmt.Sprintf("LinkedIn token exchange failed: %s", string(body)))
	}

	var resp TokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, utils.InternalServerError("Failed to parse LinkedIn token response")
	}
	resp.AccessToken = common.NormalizeToken(resp.AccessToken)
	return &resp, nil
}

func (a *OAuthAdapter) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", a.config.ClientID)
	data.Set("client_secret", a.config.ClientSecret)

	body, status, err := a.http.PostForm(ctx, "https://www.linkedin.com/oauth/v2/accessToken", data, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, utils.BadRequest(fmt.Sprintf("LinkedIn token refresh failed: %s", string(body)))
	}

	var resp TokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, utils.InternalServerError("Failed to parse refreshed token response")
	}
	resp.AccessToken = common.NormalizeToken(resp.AccessToken)
	return &resp, nil
}

func (a *OAuthAdapter) FetchUserProfile(ctx context.Context, accessToken string) (*UserProfileResponse, error) {
	token := common.NormalizeToken(accessToken)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}

	// Try OpenID userinfo endpoint
	body, status, err := a.http.Get(ctx, "https://api.linkedin.com/v2/userinfo", nil, headers)
	if err == nil && status < 400 {
		var profile UserProfileResponse
		if err := json.Unmarshal(body, &profile); err == nil && profile.Sub != "" {
			return &profile, nil
		}
	}

	// Fallback to legacy /v2/me endpoint
	body, status, err = a.http.Get(ctx, "https://api.linkedin.com/v2/me", nil, headers)
	if err != nil || status >= 400 {
		return nil, utils.BadRequest("Failed to fetch LinkedIn user profile")
	}

	var legacyProfile UserProfileResponse
	if err := json.Unmarshal(body, &legacyProfile); err != nil {
		return nil, utils.InternalServerError("Failed to parse legacy LinkedIn profile")
	}
	if legacyProfile.Sub == "" {
		legacyProfile.Sub = legacyProfile.ID
	}
	if legacyProfile.Name == "" {
		legacyProfile.Name = strings.TrimSpace(fmt.Sprintf("%s %s", legacyProfile.LocalizedFirstName, legacyProfile.LocalizedLastName))
	}
	return &legacyProfile, nil
}

func (a *OAuthAdapter) normalizeOrganizationID(val any) string {
	if val == nil {
		return ""
	}
	if m, ok := val.(map[string]any); ok {
		for _, key := range []string{"id", "organization", "organizationTarget", "organizationalTarget", "entity", "urn", "value"} {
			if v, exists := m[key]; exists && v != nil {
				res := a.normalizeOrganizationID(v)
				if res != "" {
					return res
				}
			}
		}
		return ""
	}
	s := fmt.Sprintf("%v", val)
	s = regexp.MustCompile(`^urn:li:(organization|company):`).ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func (a *OAuthAdapter) isApprovedState(state any) bool {
	if state == nil {
		return false
	}
	s := ""
	if str, ok := state.(string); ok {
		s = str
	} else if m, ok := state.(map[string]any); ok {
		if v, ok := m["value"].(string); ok {
			s = v
		}
	}
	return strings.Contains(strings.ToUpper(strings.TrimSpace(s)), "APPROVED")
}

func (a *OAuthAdapter) extractLocalizedName(val any) string {
	if val == nil {
		return ""
	}
	if str, ok := val.(string); ok && str != "" {
		return str
	}
	if m, ok := val.(map[string]any); ok {
		if loc, ok := m["localized"].(map[string]any); ok {
			for _, v := range loc {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return s
				}
			}
		}
	}
	return ""
}

func (a *OAuthAdapter) extractOrganizationLogo(logoData map[string]any) string {
	if logoData == nil {
		return ""
	}
	orig, ok := logoData["original~"].(map[string]any)
	if !ok {
		return ""
	}
	elements, ok := orig["elements"].([]any)
	if !ok || len(elements) == 0 {
		return ""
	}
	first, ok := elements[0].(map[string]any)
	if !ok {
		return ""
	}
	identifiers, ok := first["identifiers"].([]any)
	if !ok || len(identifiers) == 0 {
		return ""
	}
	idObj, ok := identifiers[0].(map[string]any)
	if !ok {
		return ""
	}
	idURL, _ := idObj["identifier"].(string)
	return idURL
}

func (a *OAuthAdapter) FetchOrganizationDetails(ctx context.Context, accessToken, orgID string) (*OrganizationDetail, error) {
	token := common.NormalizeToken(accessToken)
	headers := map[string]string{
		"Authorization":             fmt.Sprintf("Bearer %s", token),
		"X-Restli-Protocol-Version": "2.0.0",
		"LinkedIn-Version":          "202405",
	}

	params := url.Values{}
	params.Set("projection", "(id,localizedName,vanityName,logoV2(original~:playableStreams))")

	reqURL := fmt.Sprintf("https://api.linkedin.com/v2/organizations/%s", orgID)
	body, status, err := a.http.Get(ctx, reqURL, params, headers)
	if err == nil && status < 400 {
		var detail OrganizationDetail
		if err := json.Unmarshal(body, &detail); err == nil {
			return &detail, nil
		}
	}

	// Try secondary rest endpoint
	headers["Linkedin-Version"] = a.config.Version
	reqURL2 := fmt.Sprintf("https://api.linkedin.com/rest/organizations/%s", orgID)
	body2, status2, err2 := a.http.Get(ctx, reqURL2, nil, headers)
	if err2 == nil && status2 < 400 {
		var detail OrganizationDetail
		if err := json.Unmarshal(body2, &detail); err == nil {
			return &detail, nil
		}
	}

	return nil, nil
}

func (a *OAuthAdapter) FetchManagedPages(ctx context.Context, accessToken string, profile *UserProfileResponse) ([]ManagedPage, error) {
	token := common.NormalizeToken(accessToken)
	headers := map[string]string{
		"Authorization":             fmt.Sprintf("Bearer %s", token),
		"X-Restli-Protocol-Version": "2.0.0",
	}

	// Fetch organization ACLs with pagination
	allElements := make([]OrganizationACLElement, 0)
	pageSize := 100
	maxPages := 20
	start := 0

	for page := 0; page < maxPages; page++ {
		params := url.Values{}
		params.Set("q", "roleAssignee")
		params.Set("count", strconv.Itoa(pageSize))
		params.Set("start", strconv.Itoa(start))

		body, status, err := a.http.Get(ctx, "https://api.linkedin.com/v2/organizationAcls", params, headers)
		if err != nil || status >= 400 {
			break
		}

		var aclResp OrganizationACLResponse
		if err := json.Unmarshal(body, &aclResp); err != nil {
			break
		}

		nextElements := aclResp.Elements
		if len(nextElements) == 0 {
			nextElements = aclResp.Results
		}
		if len(nextElements) == 0 {
			nextElements = aclResp.Data
		}

		if len(nextElements) == 0 {
			break
		}

		allElements = append(allElements, nextElements...)
		if len(nextElements) < pageSize || (aclResp.Paging.Total > 0 && start+pageSize >= aclResp.Paging.Total) {
			break
		}
		start += pageSize
	}

	if len(allElements) == 0 {
		return []ManagedPage{}, nil
	}

	pages := make([]ManagedPage, 0)
	seenOrgIDs := make(map[string]struct{})
	orgCache := sync.Map{}

	for _, elem := range allElements {
		if !a.isApprovedState(elem.State) {
			continue
		}

		orgID := a.normalizeOrganizationID(elem.OrgExpanded)
		if orgID == "" {
			orgID = a.normalizeOrganizationID(elem.Organization)
		}
		if orgID == "" {
			orgID = a.normalizeOrganizationID(elem.OrganizationTarget)
		}
		if orgID == "" {
			orgID = a.normalizeOrganizationID(elem.OrganizationalTarget)
		}

		if orgID == "" {
			continue
		}
		if _, exists := seenOrgIDs[orgID]; exists {
			continue
		}
		seenOrgIDs[orgID] = struct{}{}

		var orgDetail *OrganizationDetail
		if cached, ok := orgCache.Load(orgID); ok {
			orgDetail = cached.(*OrganizationDetail)
		} else {
			detail, _ := a.FetchOrganizationDetails(ctx, token, orgID)
			if detail != nil {
				orgCache.Store(orgID, detail)
				orgDetail = detail
			}
		}

		orgName := ""
		logoURL := ""
		if orgDetail != nil {
			orgName = a.extractLocalizedName(orgDetail.LocalizedName)
			if orgName == "" {
				orgName = orgDetail.VanityName
			}
			logoURL = a.extractOrganizationLogo(orgDetail.LogoV2)
		}
		if orgName == "" {
			orgName = "LinkedIn Organization"
		}

		pages = append(pages, ManagedPage{
			ID:             orgID,
			PlatformID:     orgID,
			Username:       orgName,
			Name:           orgName,
			DisplayName:    orgName,
			ProfilePicture: logoURL,
			AccountType:    "organization",
			PlatformMetadata: map[string]any{
				"role":            elem.Role,
				"state":           elem.State,
				"organizationUrn": fmt.Sprintf("urn:li:organization:%s", orgID),
			},
		})
	}

	return pages, nil
}

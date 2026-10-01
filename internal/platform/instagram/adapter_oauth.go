package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/platform/common"
	"xsocial-backend/internal/utils"
)

const (
	InstagramGraphBase = "https://graph.instagram.com"
	InstagramAPIBase   = "https://api.instagram.com"
	InstagramGraphVer  = "v24.0"
)

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

func (a *OAuthAdapter) BuildAuthorizeURL(state string, scopes []string) string {
	if len(scopes) == 0 {
		scopes = []string{
			"instagram_business_basic",
			"instagram_business_manage_comments",
			"instagram_business_manage_messages",
		}
	}

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", a.config.ClientID)
	q.Set("redirect_uri", a.config.RedirectURI)
	q.Set("state", state)
	q.Set("scope", strings.Join(scopes, ","))

	return fmt.Sprintf("https://www.instagram.com/oauth/authorize?%s", q.Encode())
}

func (a *OAuthAdapter) ExchangeCodeForToken(ctx context.Context, code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", a.config.ClientID)
	data.Set("client_secret", a.config.ClientSecret)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", a.config.RedirectURI)
	data.Set("code", code)

	body, status, err := a.http.PostForm(ctx, InstagramAPIBase+"/oauth/access_token", data, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		var apiErr APIErrorResponse
		_ = json.Unmarshal(body, &apiErr)
		msg := apiErr.Error.Message
		if msg == "" {
			msg = string(body)
		}
		return nil, utils.BadRequest(fmt.Sprintf("Instagram token exchange failed: %s", msg))
	}

	var resp TokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, utils.InternalServerError("Failed to parse Instagram token response")
	}
	resp.AccessToken = common.NormalizeToken(resp.AccessToken)
	return &resp, nil
}

func (a *OAuthAdapter) ExchangeForLongLivedToken(ctx context.Context, shortLivedToken string) (*TokenResponse, error) {
	token := common.NormalizeToken(shortLivedToken)
	params := url.Values{}
	params.Set("grant_type", "ig_exchange_token")
	params.Set("client_secret", a.config.ClientSecret)
	params.Set("access_token", token)

	body, status, err := a.http.Get(ctx, InstagramGraphBase+"/access_token", params, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, utils.BadRequest("Instagram long-lived token exchange failed")
	}

	var resp TokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, utils.InternalServerError("Failed to parse long-lived token response")
	}
	resp.AccessToken = common.NormalizeToken(resp.AccessToken)
	return &resp, nil
}

func (a *OAuthAdapter) RefreshToken(ctx context.Context, longLivedToken string) (*TokenResponse, error) {
	token := common.NormalizeToken(longLivedToken)
	params := url.Values{}
	params.Set("grant_type", "ig_refresh_token")
	params.Set("access_token", token)

	body, status, err := a.http.Get(ctx, InstagramGraphBase+"/refresh_access_token", params, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, utils.BadRequest("Instagram token refresh failed")
	}

	var resp TokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, utils.InternalServerError("Failed to parse refreshed token response")
	}
	resp.AccessToken = common.NormalizeToken(resp.AccessToken)
	return &resp, nil
}

func (a *OAuthAdapter) FetchUserProfile(ctx context.Context, accessToken string, tokenUserID string) (*UserProfileResponse, error) {
	token := common.NormalizeToken(accessToken)
	params := url.Values{}
	params.Set("fields", "id,username,name,account_type,profile_picture_url")
	params.Set("access_token", token)

	reqURL := fmt.Sprintf("%s/%s/me", InstagramGraphBase, InstagramGraphVer)
	body, status, err := a.http.Get(ctx, reqURL, params, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, utils.BadRequest("Failed to fetch Instagram user profile")
	}

	var profile UserProfileResponse
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, utils.InternalServerError("Failed to parse Instagram profile")
	}

	if profile.ID == "" && tokenUserID != "" {
		profile.ID = tokenUserID
	}
	if profile.Name == "" {
		profile.Name = profile.Username
	}

	return &profile, nil
}

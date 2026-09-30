package linkedin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/models"
	"xsocial-backend/internal/platform/common"
)

const (
	LinkedInAPIBase = "https://api.linkedin.com"
)

type PublishAdapter struct {
	version string
	http    *common.HTTPClient
}

func NewPublishAdapter(cfg config.OAuthPlatformConfig) *PublishAdapter {
	ver := cfg.Version
	if ver == "" {
		ver = "202607"
	}
	return &PublishAdapter{
		version: ver,
		http:    common.NewHTTPClient(60 * time.Second),
	}
}

func (a *PublishAdapter) resolveImage(post *models.Post) *common.ParsedImagePayload {
	for _, m := range post.Media {
		mediaType := strings.ToLower(strings.TrimSpace(m.Type))
		if mediaType != "image" && mediaType != "gif" {
			continue
		}
		if m.Base64Data != "" {
			parsed := common.ParseBase64Image(m.Base64Data)
			if parsed != nil && len(parsed.Buffer) > 0 {
				if m.MimeType != "" {
					parsed.MimeType = m.MimeType
				}
				return parsed
			}
		}
	}
	return nil
}

func (a *PublishAdapter) uploadImageAsset(ctx context.Context, accessToken, authorURN string, img *common.ParsedImagePayload) (string, error) {
	token := common.NormalizeToken(accessToken)
	headers := map[string]string{
		"Authorization":             fmt.Sprintf("Bearer %s", token),
		"X-Restli-Protocol-Version": "2.0.0",
		"LinkedIn-Version":          a.version,
	}

	registerPayload := map[string]any{
		"registerUploadRequest": map[string]any{
			"owner": authorURN,
			"recipes": []string{
				"urn:li:digitalmediaRecipe:feedshare-image",
			},
			"serviceRelationships": []map[string]any{
				{
					"relationshipType": "OWNER",
					"identifier":       "urn:li:userGeneratedContent",
				},
			},
		},
	}

	reqURL := fmt.Sprintf("%s/v2/assets?action=registerUpload", LinkedInAPIBase)
	body, status, err := a.http.PostJSON(ctx, reqURL, registerPayload, headers)
	if err != nil || status >= 400 {
		return "", fmt.Errorf("LinkedIn image upload registration failed: %s", string(body))
	}

	var regResp struct {
		Value struct {
			Asset           string `json:"asset"`
			UploadMechanism struct {
				MediaUpload struct {
					UploadURL string `json:"uploadUrl"`
				} `json:"com.linkedin.digitalmedia.uploading.MediaUploadHttpRequest"`
			} `json:"uploadMechanism"`
		} `json:"value"`
	}

	if err := json.Unmarshal(body, &regResp); err != nil {
		return "", fmt.Errorf("failed to parse upload registration response: %w", err)
	}

	uploadURL := regResp.Value.UploadMechanism.MediaUpload.UploadURL
	assetURN := strings.TrimSpace(regResp.Value.Asset)

	if uploadURL == "" || assetURN == "" {
		return "", fmt.Errorf("missing upload URL or asset URN from LinkedIn")
	}

	// Upload the raw image binary
	uploadHeaders := map[string]string{}
	_, putStatus, err := a.http.PutRaw(ctx, uploadURL, img.Buffer, img.MimeType, uploadHeaders)
	if err != nil || putStatus >= 400 {
		return "", fmt.Errorf("failed to upload image binary to LinkedIn")
	}

	return assetURN, nil
}

func (a *PublishAdapter) Publish(ctx context.Context, post *models.Post, handle *models.SocialHandle) (*models.PlatformPublishResponse, error) {
	caption := strings.TrimSpace(post.Caption)
	imagePayload := a.resolveImage(post)

	hasRequestedImage := false
	for _, m := range post.Media {
		mediaType := strings.ToLower(strings.TrimSpace(m.Type))
		if mediaType == "image" || mediaType == "gif" {
			hasRequestedImage = true
			break
		}
	}

	if caption == "" && imagePayload == nil {
		return &models.PlatformPublishResponse{
			Platform: "linkedin",
			Status:   "failed",
			Message:  "LinkedIn publishing requires caption or an image with base64 data",
		}, nil
	}

	if hasRequestedImage && imagePayload == nil {
		return &models.PlatformPublishResponse{
			Platform: "linkedin",
			Status:   "failed",
			Message:  "LinkedIn image publishing requires base64 image data in post media",
		}, nil
	}

	accountType := ""
	if handle.PlatformMetadata != nil {
		if at, ok := handle.PlatformMetadata["accountType"].(string); ok {
			accountType = at
		}
	}

	isOrganization := models.IsLinkedInOrganization(accountType)
	var authorURN string
	if isOrganization {
		if orgURN, ok := handle.PlatformMetadata["organizationUrn"].(string); ok && orgURN != "" {
			authorURN = orgURN
		} else {
			authorURN = fmt.Sprintf("urn:li:organization:%s", handle.PlatformID)
		}
	} else {
		authorURN = fmt.Sprintf("urn:li:person:%s", handle.PlatformID)
	}

	token := common.NormalizeToken(handle.PlatformAccessToken)
	var mediaAssetURN string
	if imagePayload != nil {
		asset, err := a.uploadImageAsset(ctx, token, authorURN, imagePayload)
		if err != nil {
			return &models.PlatformPublishResponse{
				Platform: "linkedin",
				Status:   "failed",
				Message:  err.Error(),
			}, nil
		}
		mediaAssetURN = asset
	}

	mediaCategory := "NONE"
	if mediaAssetURN != "" {
		mediaCategory = "IMAGE"
	}

	shareContent := map[string]any{
		"shareCommentary": map[string]any{
			"text": caption,
		},
		"shareMediaCategory": mediaCategory,
	}

	if mediaAssetURN != "" {
		shareContent["media"] = []map[string]any{
			{
				"status": "READY",
				"media":  mediaAssetURN,
				"description": map[string]any{
					"text": caption,
				},
				"title": map[string]any{
					"text": post.Title,
				},
			},
		}
	}

	payload := map[string]any{
		"author":         authorURN,
		"lifecycleState": "PUBLISHED",
		"specificContent": map[string]any{
			"com.linkedin.ugc.ShareContent": shareContent,
		},
		"visibility": map[string]any{
			"com.linkedin.ugc.MemberNetworkVisibility": "PUBLIC",
		},
	}

	headers := map[string]string{
		"Authorization":             fmt.Sprintf("Bearer %s", token),
		"X-Restli-Protocol-Version": "2.0.0",
		"LinkedIn-Version":          a.version,
	}

	reqURL := fmt.Sprintf("%s/v2/ugcPosts", LinkedInAPIBase)
	body, status, err := a.http.PostJSON(ctx, reqURL, payload, headers)
	if err != nil || status >= 400 {
		var rawErr any
		_ = json.Unmarshal(body, &rawErr)
		return &models.PlatformPublishResponse{
			Platform: "linkedin",
			Status:   "failed",
			Message:  fmt.Sprintf("LinkedIn publish failed: %s", string(body)),
			Raw:      rawErr,
		}, nil
	}

	var pubResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &pubResp)

	var rawResp any
	_ = json.Unmarshal(body, &rawResp)

	return &models.PlatformPublishResponse{
		Platform:       "linkedin",
		Status:         "success",
		PlatformPostID: pubResp.ID,
		Raw:            rawResp,
	}, nil
}

func (a *PublishAdapter) DeletePost(ctx context.Context, accessToken, platformPostID string) error {
	urn := strings.TrimSpace(platformPostID)
	if urn == "" {
		return nil
	}

	token := common.NormalizeToken(accessToken)
	encodedURN := url.PathEscape(urn)
	reqURL := fmt.Sprintf("%s/v2/ugcPosts/%s", LinkedInAPIBase, encodedURN)

	headers := map[string]string{
		"Authorization":             fmt.Sprintf("Bearer %s", token),
		"X-Restli-Protocol-Version": "2.0.0",
		"LinkedIn-Version":          a.version,
	}

	_, _, err := a.http.Delete(ctx, reqURL, headers)
	return err
}

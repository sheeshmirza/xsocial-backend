package linkedin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
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

func (a *PublishAdapter) resolveImages(ctx context.Context, post *models.Post) []*common.ParsedImagePayload {
	var images []*common.ParsedImagePayload
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
				images = append(images, parsed)
				continue
			}
		}

		if strings.TrimSpace(m.URL) != "" {
			buf, contentType, err := a.http.FetchMediaBytes(ctx, m.URL)
			if err == nil && len(buf) > 0 {
				mime := m.MimeType
				if mime == "" {
					mime = contentType
				}
				if mime == "" {
					mime = "image/jpeg"
				}
				images = append(images, &common.ParsedImagePayload{
					MimeType: mime,
					Buffer:   buf,
				})
			}
		}
	}
	return images
}

func (a *PublishAdapter) resolveVideos(ctx context.Context, post *models.Post) []*common.ParsedImagePayload {
	var videos []*common.ParsedImagePayload
	for _, m := range post.Media {
		mediaType := strings.ToLower(strings.TrimSpace(m.Type))
		if mediaType != "video" {
			continue
		}

		if m.Base64Data != "" {
			parsed := common.ParseBase64Image(m.Base64Data)
			if parsed != nil && len(parsed.Buffer) > 0 {
				if m.MimeType != "" {
					parsed.MimeType = m.MimeType
				}
				videos = append(videos, parsed)
				continue
			}
		}

		if strings.TrimSpace(m.URL) != "" {
			buf, contentType, err := a.http.FetchMediaBytes(ctx, m.URL)
			if err == nil && len(buf) > 0 {
				mime := m.MimeType
				if mime == "" {
					mime = contentType
				}
				if mime == "" {
					mime = "video/mp4"
				}
				videos = append(videos, &common.ParsedImagePayload{
					MimeType: mime,
					Buffer:   buf,
				})
			}
		}
	}
	return videos
}

func (a *PublishAdapter) uploadMediaAsset(ctx context.Context, accessToken, authorURN, recipe string, media *common.ParsedImagePayload) (string, error) {
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
				recipe,
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
		return "", fmt.Errorf("LinkedIn media upload registration failed: %s", string(body))
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

	// Upload raw binary bytes
	uploadHeaders := map[string]string{}
	_, putStatus, err := a.http.PutRaw(ctx, uploadURL, media.Buffer, media.MimeType, uploadHeaders)
	if err != nil || putStatus >= 400 {
		return "", fmt.Errorf("failed to upload media binary to LinkedIn")
	}

	return assetURN, nil
}

func (a *PublishAdapter) Publish(ctx context.Context, post *models.Post, handle *models.SocialHandle) (*models.PlatformPublishResponse, error) {
	caption := strings.TrimSpace(post.Caption)
	postType := strings.ToLower(strings.TrimSpace(post.PostType))

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
	if token == "" {
		return &models.PlatformPublishResponse{
			Platform: "linkedin",
			Status:   "failed",
			Message:  "LinkedIn access token is missing",
		}, nil
	}

	// Resolve media
	imagePayloads := a.resolveImages(ctx, post)
	videoPayloads := a.resolveVideos(ctx, post)

	// Validate inputs
	if caption == "" && len(imagePayloads) == 0 && len(videoPayloads) == 0 && postType != "article" && postType != "link" {
		return &models.PlatformPublishResponse{
			Platform: "linkedin",
			Status:   "failed",
			Message:  "LinkedIn publishing requires caption, image, or video media",
		}, nil
	}

	shareContent := map[string]any{
		"shareCommentary": map[string]any{
			"text": caption,
		},
	}

	// Native LinkedIn Reshare (quote post or direct share)
	if post.IsReshare && strings.HasPrefix(post.OriginPlatformPostID, "urn:li:") {
		shareContent["resharedShare"] = post.OriginPlatformPostID
	}

	// Determine posting category:
	// 1. Article / Link post
	isArticle := postType == "article" || postType == "link"
	var linkURL string
	if isArticle {
		// Detect URL from media or caption
		for _, m := range post.Media {
			if strings.ToLower(m.Type) == "link" && m.URL != "" {
				linkURL = m.URL
				break
			}
		}
		if linkURL == "" {
			reURL := regexp.MustCompile(`https?://[^\s]+`)
			linkURL = reURL.FindString(caption)
		}
	}

	if isArticle && linkURL != "" {
		shareContent["shareMediaCategory"] = "ARTICLE"
		shareContent["media"] = []map[string]any{
			{
				"status":      "READY",
				"originalUrl": linkURL,
				"description": map[string]any{
					"text": caption,
				},
				"title": map[string]any{
					"text": post.Title,
				},
			},
		}
	} else if len(videoPayloads) > 0 || postType == "video" {
		// 2. Video Post
		if len(videoPayloads) == 0 {
			return &models.PlatformPublishResponse{
				Platform: "linkedin",
				Status:   "failed",
				Message:  "Video media binary could not be resolved for LinkedIn video post",
			}, nil
		}
		videoAssetURN, err := a.uploadMediaAsset(ctx, token, authorURN, "urn:li:digitalmediaRecipe:feedshare-video", videoPayloads[0])
		if err != nil {
			return &models.PlatformPublishResponse{
				Platform: "linkedin",
				Status:   "failed",
				Message:  err.Error(),
			}, nil
		}
		shareContent["shareMediaCategory"] = "VIDEO"
		shareContent["media"] = []map[string]any{
			{
				"status": "READY",
				"media":  videoAssetURN,
				"description": map[string]any{
					"text": caption,
				},
				"title": map[string]any{
					"text": post.Title,
				},
			},
		}
	} else if len(imagePayloads) > 0 {
		// 3. Image or Multi-Image (Carousel) Post
		mediaList := make([]map[string]any, 0, len(imagePayloads))
		for i, img := range imagePayloads {
			assetURN, err := a.uploadMediaAsset(ctx, token, authorURN, "urn:li:digitalmediaRecipe:feedshare-image", img)
			if err != nil {
				return &models.PlatformPublishResponse{
					Platform: "linkedin",
					Status:   "failed",
					Message:  fmt.Sprintf("Failed uploading image slide %d: %v", i+1, err),
				}, nil
			}
			mediaList = append(mediaList, map[string]any{
				"status": "READY",
				"media":  assetURN,
				"description": map[string]any{
					"text": caption,
				},
				"title": map[string]any{
					"text": fmt.Sprintf("%s - %d", post.Title, i+1),
				},
			})
		}
		shareContent["shareMediaCategory"] = "IMAGE"
		shareContent["media"] = mediaList
	} else {
		// 4. Text-only Post
		shareContent["shareMediaCategory"] = "NONE"
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

	// Post first comment if provided
	var rawComment any
	if pubResp.ID != "" && strings.TrimSpace(post.FirstComment) != "" {
		encodedURN := url.QueryEscape(pubResp.ID)
		commentURL := fmt.Sprintf("%s/v2/socialActions/%s/comments", LinkedInAPIBase, encodedURN)
		commentPayload := map[string]any{
			"actor": authorURN,
			"message": map[string]any{
				"text": strings.TrimSpace(post.FirstComment),
			},
		}
		cBody, cStatus, cErr := a.http.PostJSON(ctx, commentURL, commentPayload, headers)
		if cErr == nil && cStatus < 400 {
			_ = json.Unmarshal(cBody, &rawComment)
		}
	}

	return &models.PlatformPublishResponse{
		Platform:       "linkedin",
		Status:         "success",
		PlatformPostID: pubResp.ID,
		Raw: map[string]any{
			"response": rawResp,
			"comment":  rawComment,
		},
	}, nil
}

func (a *PublishAdapter) DeletePost(ctx context.Context, accessToken, platformPostID string) error {
	urn := strings.TrimSpace(platformPostID)
	if urn == "" {
		return nil
	}

	token := common.NormalizeToken(accessToken)
	encodedURN := url.QueryEscape(urn)
	reqURL := fmt.Sprintf("%s/v2/ugcPosts/%s", LinkedInAPIBase, encodedURN)

	headers := map[string]string{
		"Authorization":             fmt.Sprintf("Bearer %s", token),
		"X-Restli-Protocol-Version": "2.0.0",
		"LinkedIn-Version":          a.version,
	}

	_, _, err := a.http.Delete(ctx, reqURL, headers)
	return err
}

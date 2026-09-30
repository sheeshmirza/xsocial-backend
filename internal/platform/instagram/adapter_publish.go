package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/models"
	"xsocial-backend/internal/platform/common"
)

type PublishAdapter struct {
	http *common.HTTPClient
}

func NewPublishAdapter() *PublishAdapter {
	return &PublishAdapter{
		http: common.NewHTTPClient(60 * time.Second),
	}
}

func (a *PublishAdapter) isVideo(item models.MediaItem) bool {
	return strings.ToLower(strings.TrimSpace(item.Type)) == "video"
}

func (a *PublishAdapter) createChildContainer(ctx context.Context, igUserID, token string, item models.MediaItem) (string, error) {
	params := url.Values{}
	params.Set("access_token", token)
	params.Set("is_carousel_item", "true")

	if a.isVideo(item) {
		params.Set("media_type", "VIDEO")
		params.Set("video_url", strings.TrimSpace(item.URL))
	} else {
		params.Set("image_url", strings.TrimSpace(item.URL))
	}

	reqURL := fmt.Sprintf("%s/%s/%s/media", InstagramGraphBase, InstagramGraphVer, igUserID)
	body, status, err := a.http.PostParams(ctx, reqURL, params, nil)
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", fmt.Errorf("child container creation failed: %s", string(body))
	}

	var resp ContainerResponse
	if err := json.Unmarshal(body, &resp); err != nil || resp.ID == "" {
		return "", fmt.Errorf("invalid child container response: %s", string(body))
	}
	return resp.ID, nil
}

func (a *PublishAdapter) waitForContainerReady(ctx context.Context, token, containerID string, maxAttempts int, delay time.Duration) (bool, string) {
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	if delay <= 0 {
		delay = 2 * time.Second
	}

	reqURL := fmt.Sprintf("%s/%s/%s", InstagramGraphBase, InstagramGraphVer, containerID)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		params := url.Values{}
		params.Set("access_token", token)
		params.Set("fields", "status_code")

		body, status, err := a.http.Get(ctx, reqURL, params, nil)
		if err == nil && status < 400 {
			var resp ContainerStatusResponse
			if err := json.Unmarshal(body, &resp); err == nil {
				sc := strings.ToUpper(resp.StatusCode)
				if sc == "FINISHED" {
					return true, sc
				}
				if sc == "ERROR" || sc == "EXPIRED" {
					return false, sc
				}
			}
		}

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return false, "CANCELLED"
			case <-time.After(delay):
			}
		}
	}
	return false, "IN_PROGRESS"
}

func (a *PublishAdapter) publishWithToken(ctx context.Context, igUserID, token string, post *models.Post, mediaList []models.MediaItem) (*models.PlatformPublishResponse, error) {
	caption := strings.TrimSpace(post.Caption)
	postType := strings.ToLower(strings.TrimSpace(post.PostType))
	isStory := postType == "story" || postType == "stories"
	isCarousel := len(mediaList) > 1 || postType == "carousel"

	var containerID string
	var rawContainer any

	if isStory {
		// Story container (image or video without caption)
		first := mediaList[0]
		params := url.Values{}
		params.Set("access_token", token)
		params.Set("media_type", "STORIES")
		if a.isVideo(first) {
			params.Set("video_url", strings.TrimSpace(first.URL))
		} else {
			params.Set("image_url", strings.TrimSpace(first.URL))
		}

		reqURL := fmt.Sprintf("%s/%s/%s/media", InstagramGraphBase, InstagramGraphVer, igUserID)
		body, status, err := a.http.PostParams(ctx, reqURL, params, nil)
		if err != nil || status >= 400 {
			return &models.PlatformPublishResponse{
				Platform: "instagram",
				Status:   "failed",
				Message:  fmt.Sprintf("Instagram stories container creation failed: %s", string(body)),
			}, nil
		}

		var resp ContainerResponse
		_ = json.Unmarshal(body, &resp)
		_ = json.Unmarshal(body, &rawContainer)
		containerID = resp.ID
	} else if isCarousel {
		// Carousel container (multi-photo and/or video)
		childIDs := make([]string, 0, len(mediaList))
		for _, item := range mediaList {
			childID, err := a.createChildContainer(ctx, igUserID, token, item)
			if err != nil {
				return &models.PlatformPublishResponse{
					Platform: "instagram",
					Status:   "failed",
					Message:  fmt.Sprintf("Instagram carousel item creation failed: %v", err),
				}, nil
			}
			childIDs = append(childIDs, childID)
		}

		params := url.Values{}
		params.Set("access_token", token)
		params.Set("caption", caption)
		params.Set("media_type", "CAROUSEL")
		params.Set("children", strings.Join(childIDs, ","))

		reqURL := fmt.Sprintf("%s/%s/%s/media", InstagramGraphBase, InstagramGraphVer, igUserID)
		body, status, err := a.http.PostParams(ctx, reqURL, params, nil)
		if err != nil || status >= 400 {
			return &models.PlatformPublishResponse{
				Platform: "instagram",
				Status:   "failed",
				Message:  fmt.Sprintf("Instagram carousel container creation failed: %s", string(body)),
			}, nil
		}

		var resp ContainerResponse
		_ = json.Unmarshal(body, &resp)
		_ = json.Unmarshal(body, &rawContainer)
		containerID = resp.ID
	} else {
		// Single Feed Image or Reel
		first := mediaList[0]
		params := url.Values{}
		params.Set("access_token", token)
		params.Set("caption", caption)

		if a.isVideo(first) || postType == "reel" || postType == "reels" || postType == "video" {
			params.Set("media_type", "REELS")
			params.Set("video_url", strings.TrimSpace(first.URL))
			params.Set("share_to_feed", "true")
		} else {
			params.Set("image_url", strings.TrimSpace(first.URL))
		}

		reqURL := fmt.Sprintf("%s/%s/%s/media", InstagramGraphBase, InstagramGraphVer, igUserID)
		body, status, err := a.http.PostParams(ctx, reqURL, params, nil)
		if err != nil || status >= 400 {
			return &models.PlatformPublishResponse{
				Platform: "instagram",
				Status:   "failed",
				Message:  fmt.Sprintf("Instagram single container creation failed: %s", string(body)),
			}, nil
		}

		var resp ContainerResponse
		_ = json.Unmarshal(body, &resp)
		_ = json.Unmarshal(body, &rawContainer)
		containerID = resp.ID
	}

	if containerID == "" {
		return &models.PlatformPublishResponse{
			Platform: "instagram",
			Status:   "failed",
			Message:  "Instagram container creation failed: no container ID returned",
			Raw:      rawContainer,
		}, nil
	}

	ready, sc := a.waitForContainerReady(ctx, token, containerID, 10, 2*time.Second)
	if !ready {
		return &models.PlatformPublishResponse{
			Platform: "instagram",
			Status:   "failed",
			Message:  fmt.Sprintf("Instagram media was not ready for publishing (status: %s)", sc),
			Raw:      rawContainer,
		}, nil
	}

	publishParams := url.Values{}
	publishParams.Set("access_token", token)
	publishParams.Set("creation_id", containerID)

	publishURL := fmt.Sprintf("%s/%s/%s/media_publish", InstagramGraphBase, InstagramGraphVer, igUserID)
	pubBody, pubStatus, err := a.http.PostParams(ctx, publishURL, publishParams, nil)
	if err != nil || pubStatus >= 400 {
		return &models.PlatformPublishResponse{
			Platform: "instagram",
			Status:   "failed",
			Message:  fmt.Sprintf("Instagram media publish failed: %s", string(pubBody)),
			Raw:      rawContainer,
		}, nil
	}

	var pubResp PublishMediaResponse
	_ = json.Unmarshal(pubBody, &pubResp)

	postURL := ""
	if pubResp.ID != "" {
		postURL = fmt.Sprintf("https://www.instagram.com/p/%s/", pubResp.ID)
	}

	var rawPub any
	_ = json.Unmarshal(pubBody, &rawPub)

	// Post first comment if provided
	var rawComment any
	if pubResp.ID != "" && strings.TrimSpace(post.FirstComment) != "" {
		commentParams := url.Values{}
		commentParams.Set("access_token", token)
		commentParams.Set("message", strings.TrimSpace(post.FirstComment))
		commentURL := fmt.Sprintf("%s/%s/%s/comments", InstagramGraphBase, InstagramGraphVer, pubResp.ID)
		commentBody, commentStatus, cErr := a.http.PostParams(ctx, commentURL, commentParams, nil)
		if cErr == nil && commentStatus < 400 {
			_ = json.Unmarshal(commentBody, &rawComment)
		}
	}

	return &models.PlatformPublishResponse{
		Platform:       "instagram",
		Status:         "success",
		PlatformPostID: pubResp.ID,
		URL:            postURL,
		Raw: map[string]any{
			"container": rawContainer,
			"publish":   rawPub,
			"comment":   rawComment,
		},
	}, nil
}

func (a *PublishAdapter) Publish(ctx context.Context, post *models.Post, handle *models.SocialHandle) (*models.PlatformPublishResponse, error) {
	igUserID := strings.TrimSpace(handle.PlatformID)
	token := common.NormalizeToken(handle.PlatformAccessToken)

	if igUserID == "" || token == "" {
		return &models.PlatformPublishResponse{
			Platform: "instagram",
			Status:   "failed",
			Message:  "Instagram account credentials are missing",
		}, nil
	}

	mediaList := make([]models.MediaItem, 0)
	for _, m := range post.Media {
		if strings.TrimSpace(m.URL) != "" {
			mediaList = append(mediaList, m)
		}
	}

	if len(mediaList) == 0 {
		return &models.PlatformPublishResponse{
			Platform: "instagram",
			Status:   "failed",
			Message:  "Instagram publishing requires at least one media URL",
		}, nil
	}

	reInaccessible := regexp.MustCompile(`(?i)^(memory|data):`)
	for _, m := range mediaList {
		if reInaccessible.MatchString(strings.TrimSpace(m.URL)) {
			return &models.PlatformPublishResponse{
				Platform: "instagram",
				Status:   "failed",
				Message:  "Instagram publishing requires a publicly accessible media URL",
			}, nil
		}
	}

	res, err := a.publishWithToken(ctx, igUserID, token, post, mediaList)
	if err != nil {
		return &models.PlatformPublishResponse{
			Platform: "instagram",
			Status:   "failed",
			Message:  err.Error(),
		}, nil
	}

	// If failed due to expired token (code 190), try refreshing if refreshToken exists
	if res.Status == "failed" && strings.Contains(strings.ToLower(res.Message), "token") {
		refreshCred := common.NormalizeToken(handle.PlatformRefreshToken)
		if refreshCred == "" {
			refreshCred = token
		}
		if refreshCred != "" {
			log.Println("[InstagramPublishAdapter] Token error detected, attempting auto-refresh...")
			oauth := NewOAuthAdapter(config.OAuthPlatformConfig{})
			refreshed, rErr := oauth.RefreshToken(ctx, refreshCred)
			if rErr == nil && refreshed.AccessToken != "" {
				handle.PlatformAccessToken = refreshed.AccessToken
				retryRes, retryErr := a.publishWithToken(ctx, igUserID, refreshed.AccessToken, post, mediaList)
				if retryErr == nil {
					return retryRes, nil
				}
			}
		}
	}

	return res, nil
}

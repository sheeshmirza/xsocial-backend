package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"xsocial-backend/internal/utils"
)

type HTTPClient struct {
	client *http.Client
}

func NewHTTPClient(timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &HTTPClient{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

func (c *HTTPClient) Get(ctx context.Context, requestURL string, params url.Values, headers map[string]string) ([]byte, int, error) {
	if len(params) > 0 {
		parsedURL, err := url.Parse(requestURL)
		if err != nil {
			return nil, 0, err
		}
		q := parsedURL.Query()
		for k, v := range params {
			for _, item := range v {
				q.Add(k, item)
			}
		}
		parsedURL.RawQuery = q.Encode()
		requestURL = parsedURL.String()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, 0, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.do(req)
}

func (c *HTTPClient) PostJSON(ctx context.Context, requestURL string, body any, headers map[string]string) ([]byte, int, error) {
	bytes, status, _, err := c.PostJSONWithRespHeaders(ctx, requestURL, body, headers)
	return bytes, status, err
}

func (c *HTTPClient) PostJSONWithRespHeaders(ctx context.Context, requestURL string, body any, headers map[string]string) ([]byte, int, http.Header, error) {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, 0, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, nil, utils.BadGateway(fmt.Sprintf("Provider request failed: %v", err))
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, resp.Header, err
	}

	if resp.StatusCode >= 400 {
		log.Printf("[PlatformHTTP] Error %d on %s %s: %s", resp.StatusCode, req.Method, req.URL.String(), string(bodyBytes))
	}

	return bodyBytes, resp.StatusCode, resp.Header, nil
}

func (c *HTTPClient) PostForm(ctx context.Context, requestURL string, data url.Values, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.do(req)
}

func (c *HTTPClient) PostParams(ctx context.Context, requestURL string, params url.Values, headers map[string]string) ([]byte, int, error) {
	if len(params) > 0 {
		parsedURL, err := url.Parse(requestURL)
		if err != nil {
			return nil, 0, err
		}
		q := parsedURL.Query()
		for k, v := range params {
			for _, item := range v {
				q.Add(k, item)
			}
		}
		parsedURL.RawQuery = q.Encode()
		requestURL = parsedURL.String()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return nil, 0, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.do(req)
}

func (c *HTTPClient) PutRaw(ctx context.Context, requestURL string, data []byte, contentType string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, requestURL, bytes.NewBuffer(data))
	if err != nil {
		return nil, 0, err
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.do(req)
}

func (c *HTTPClient) Delete(ctx context.Context, requestURL string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, requestURL, nil)
	if err != nil {
		return nil, 0, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.do(req)
}

func (c *HTTPClient) do(req *http.Request) ([]byte, int, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, utils.BadGateway(fmt.Sprintf("Provider request failed: %v", err))
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode >= 400 {
		log.Printf("[PlatformHTTP] Error %d on %s %s: %s", resp.StatusCode, req.Method, req.URL.String(), string(bodyBytes))
	}

	return bodyBytes, resp.StatusCode, nil
}

func (c *HTTPClient) FetchMediaBytes(ctx context.Context, mediaURL string) ([]byte, string, error) {
	if err := utils.ValidatePublicURL(mediaURL); err != nil {
		return nil, "", utils.BadRequest(fmt.Sprintf("media URL rejected for security: %v", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("HTTP error %d fetching media from URL", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}

	return body, contentType, nil
}


package common

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"xsocial-backend/internal/utils"
)

func NormalizeToken(token string) string {
	t := strings.TrimSpace(token)
	t = strings.Trim(t, "\"'`")
	// Remove all whitespace
	t = regexp.MustCompile(`\s+`).ReplaceAllString(t, "")
	return t
}

func GenerateOAuthState(userID string, secret string) string {
	nowMs := time.Now().UnixMilli()
	payload := fmt.Sprintf("%s:%d", userID, nowMs)
	sig := utils.HMACSHA256(secret, payload)
	if len(sig) > 32 {
		sig = sig[:32]
	}
	return fmt.Sprintf("%s:%s", payload, sig)
}

func ValidateOAuthState(state string, secret string) (string, error) {
	parts := strings.Split(state, ":")
	if len(parts) < 3 {
		return "", utils.BadRequest("Invalid OAuth state format")
	}

	uid := parts[0]
	tsStr := parts[1]
	sig := parts[2]

	tsNum, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return "", utils.BadRequest("Invalid timestamp in OAuth state")
	}

	age := time.Now().UnixMilli() - tsNum
	if age < 0 || age > 15*60*1000 {
		return "", utils.BadRequest("OAuth state expired")
	}

	payload := fmt.Sprintf("%s:%s", uid, tsStr)
	expected := utils.HMACSHA256(secret, payload)
	if len(expected) > 32 {
		expected = expected[:32]
	}

	if !utils.ConstantTimeCompare(sig, expected) {
		return "", utils.BadRequest("Invalid OAuth state signature")
	}

	return uid, nil
}

type ParsedImagePayload struct {
	MimeType string
	Buffer   []byte
}

func ParseBase64Image(dataURI string) *ParsedImagePayload {
	raw := strings.TrimSpace(dataURI)
	if raw == "" {
		return nil
	}

	re := regexp.MustCompile(`(?i)^data:([^;]+);base64,(.+)$`)
	matches := re.FindStringSubmatch(raw)

	mimeType := "image/jpeg"
	encodedBody := raw

	if len(matches) == 3 {
		mimeType = matches[1]
		encodedBody = matches[2]
	}

	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(encodedBody, "")
	if normalized == "" {
		return nil
	}

	buffer, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil || len(buffer) == 0 {
		return nil
	}

	return &ParsedImagePayload{
		MimeType: mimeType,
		Buffer:   buffer,
	}
}

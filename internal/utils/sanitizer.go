package utils

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	htmlTagRegex = regexp.MustCompile(`<[^>]*>`)
	currencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)
)

// SanitizeString strips null bytes, leading/trailing whitespace, and dangerous control characters.
func SanitizeString(s string) string {
	// Strip null bytes
	s = strings.ReplaceAll(s, "\x00", "")
	// Strip HTML tags for plain text fields to prevent stored XSS
	s = htmlTagRegex.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// SanitizeText trims whitespace and removes null bytes, preserving intentional formatting and newlines.
func SanitizeText(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.TrimSpace(s)
}

// IsValidEmail verifies standard RFC 5321 email format.
func IsValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	if strings.Contains(email, "..") {
		return false
	}
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

// IsValidTimezone verifies if the given string is a valid IANA timezone location.
func IsValidTimezone(tz string) bool {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

// IsValidCurrency verifies if the given currency is a standard 3-letter ISO 4217 code.
func IsValidCurrency(currency string) bool {
	return currencyRegex.MatchString(strings.TrimSpace(currency))
}

// ValidateURL ensures the raw URL has a valid http/https scheme and a non-empty host.
func ValidateURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return fmt.Errorf("URL cannot be empty")
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL host is missing")
	}
	return nil
}

// ValidatePublicURL checks that a URL is well-formed AND does not point to internal,
// loopback, link-local, multicast, or private IP networks (SSRF defense).
func ValidatePublicURL(rawURL string) error {
	if err := ValidateURL(rawURL); err != nil {
		return err
	}

	parsed, _ := url.Parse(rawURL)
	host := parsed.Hostname()

	// Prohibit literal localhost or cloud metadata hostnames
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || lowerHost == "metadata.google.internal" || strings.HasSuffix(lowerHost, ".local") {
		return fmt.Errorf("internal or loopback hostnames are not permitted")
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		// If DNS resolution fails, allow if it's already an IP or report error
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("could not resolve hostname: %w", err)
		}
		ips = []net.IP{ip}
	}

	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("access to private or link-local address %s is prohibited", ip.String())
		}
		// Explicitly block AWS/GCP/Azure link-local metadata address 169.254.169.254
		if ip.String() == "169.254.169.254" {
			return fmt.Errorf("access to cloud instance metadata service is prohibited")
		}
	}

	return nil
}

// MaskURL redacts passwords and sensitive query params from a URL string for safe logging.
func MaskURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "***"
	}
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(parsed.User.Username(), "redacted")
		}
	}
	q := parsed.Query()
	sensitiveKeys := []string{"access_token", "client_secret", "secret", "token", "key", "api_key", "password", "refresh_token"}
	hasSensitive := false
	for _, key := range sensitiveKeys {
		if q.Has(key) {
			q.Set(key, "redacted")
			hasSensitive = true
		}
	}
	if hasSensitive {
		parsed.RawQuery = q.Encode()
	}
	return parsed.String()
}

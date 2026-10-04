package utils_test

import (
	"strings"
	"testing"

	"xsocial-backend/internal/utils"
)

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  hello world  ", "hello world"},
		{"hello\x00world", "helloworld"},
		{"<script>alert(1)</script>hello", "alert(1)hello"},
		{"<b>bold text</b>", "bold text"},
		{"normal-title-123", "normal-title-123"},
	}

	for _, tt := range tests {
		res := utils.SanitizeString(tt.input)
		if res != tt.expected {
			t.Errorf("SanitizeString(%q) = %q; want %q", tt.input, res, tt.expected)
		}
	}
}

func TestIsValidEmail(t *testing.T) {
	valid := []string{
		"user@example.com",
		"john.doe@company.org",
		"dev+test@domain.co.uk",
		"admin@xsocial.io",
	}
	for _, email := range valid {
		if !utils.IsValidEmail(email) {
			t.Errorf("expected %q to be valid email", email)
		}
	}

	invalid := []string{
		"not-an-email",
		"@missing-user.com",
		"user@.com",
		"user@com",
		"",
		"user@domain..com",
	}
	for _, email := range invalid {
		if utils.IsValidEmail(email) {
			t.Errorf("expected %q to be invalid email", email)
		}
	}
}

func TestIsValidTimezone(t *testing.T) {
	if !utils.IsValidTimezone("UTC") {
		t.Error("expected UTC to be valid")
	}
	if !utils.IsValidTimezone("America/New_York") {
		t.Error("expected America/New_York to be valid")
	}
	if !utils.IsValidTimezone("Asia/Tokyo") {
		t.Error("expected Asia/Tokyo to be valid")
	}
	if utils.IsValidTimezone("Mars/Colony_1") {
		t.Error("expected invalid timezone to fail")
	}
	if utils.IsValidTimezone("") {
		t.Error("expected empty timezone to fail")
	}
}

func TestIsValidCurrency(t *testing.T) {
	if !utils.IsValidCurrency("USD") {
		t.Error("expected USD to be valid")
	}
	if !utils.IsValidCurrency("EUR") {
		t.Error("expected EUR to be valid")
	}
	if !utils.IsValidCurrency("GBP") {
		t.Error("expected GBP to be valid")
	}
	if utils.IsValidCurrency("usd") {
		t.Error("expected lowercase to be invalid")
	}
	if utils.IsValidCurrency("USDT") {
		t.Error("expected 4 chars to be invalid")
	}
}

func TestValidateURLAndSSRF(t *testing.T) {
	t.Run("Valid Public URLs", func(t *testing.T) {
		valid := []string{
			"https://example.com/image.jpg",
			"https://images.unsplash.com/photo-123",
			"http://cdn.example.org/media.png",
		}
		for _, u := range valid {
			if err := utils.ValidateURL(u); err != nil {
				t.Errorf("expected %q to be valid URL: %v", u, err)
			}
		}
	})

	t.Run("Invalid Schemes", func(t *testing.T) {
		invalid := []string{
			"javascript:alert(1)",
			"ftp://example.com/file",
			"data:image/png;base64,123",
			"",
		}
		for _, u := range invalid {
			if err := utils.ValidateURL(u); err == nil {
				t.Errorf("expected %q to be rejected", u)
			}
		}
	})

	t.Run("SSRF Loopback Block", func(t *testing.T) {
		ssrfURLs := []string{
			"http://localhost/admin",
			"http://127.0.0.1:8080/metrics",
			"http://169.254.169.254/latest/meta-data/",
			"http://metadata.google.internal/computeMetadata/v1/",
		}
		for _, u := range ssrfURLs {
			if err := utils.ValidatePublicURL(u); err == nil {
				t.Errorf("expected SSRF url %q to be blocked", u)
			}
		}
	})
}

func TestMaskURL(t *testing.T) {
	tests := []struct {
		input    string
		contains string
		notHas   string
	}{
		{
			input:    "redis://:supersecretpass@127.0.0.1:6379",
			contains: ":redacted@127.0.0.1:6379",
			notHas:   "supersecretpass",
		},
		{
			input:    "https://graph.instagram.com/v12.0/me?access_token=EAAB12345secret&fields=id",
			contains: "access_token=redacted",
			notHas:   "EAAB12345secret",
		},
		{
			input:    "mongodb+srv://admin:adminpassword123@cluster.mongodb.net/db",
			contains: "admin:redacted@cluster.mongodb.net/db",
			notHas:   "adminpassword123",
		},
		{
			input:    "https://api.linkedin.com/v2/oauth?client_secret=topsecretkey",
			contains: "client_secret=redacted",
			notHas:   "topsecretkey",
		},
	}

	for _, tt := range tests {
		masked := utils.MaskURL(tt.input)
		if !strings.Contains(masked, tt.contains) {
			t.Errorf("MaskURL(%q) = %q; expected to contain %q", tt.input, masked, tt.contains)
		}
		if strings.Contains(masked, tt.notHas) {
			t.Errorf("MaskURL(%q) leaked sensitive value %q in output: %q", tt.input, tt.notHas, masked)
		}
	}
}

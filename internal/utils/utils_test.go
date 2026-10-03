package utils_test

import (
	"net/http"
	"testing"

	"xsocial-backend/internal/utils"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name         string
		pageStr      string
		limitStr     string
		expectedPage int
		expectedLim  int
		expectedSkip int
	}{
		{"default values on empty", "", "", 1, 10, 0},
		{"valid page and limit", "3", "25", 3, 25, 50},
		{"negative page fallback", "-1", "20", 1, 20, 0},
		{"limit exceeds 100 capped", "1", "500", 1, 100, 0},
		{"zero limit fallback to default", "2", "0", 2, 10, 10},
		{"invalid string inputs", "invalid", "bad", 1, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, limit, skip := utils.ParsePagination(tt.pageStr, tt.limitStr)
			if page != tt.expectedPage {
				t.Errorf("expected page %d, got %d", tt.expectedPage, page)
			}
			if limit != tt.expectedLim {
				t.Errorf("expected limit %d, got %d", tt.expectedLim, limit)
			}
			if skip != tt.expectedSkip {
				t.Errorf("expected skip %d, got %d", tt.expectedSkip, skip)
			}
		})
	}
}

func TestNormalizeAccountIDs(t *testing.T) {
	input := []string{"acc-1, acc-2", "acc-3", "acc-1", "", "  acc-4  "}
	result := utils.NormalizeAccountIDs(input)

	expected := []string{"acc-1", "acc-2", "acc-3", "acc-4"}
	if len(result) != len(expected) {
		t.Fatalf("expected length %d, got %d (%v)", len(expected), len(result), result)
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("at index %d: expected %q, got %q", i, v, result[i])
		}
	}
}

func TestCryptoUtils(t *testing.T) {
	t.Run("SHA256 consistency", func(t *testing.T) {
		h1 := utils.SHA256("test-value")
		h2 := utils.SHA256("test-value")
		if h1 != h2 {
			t.Errorf("expected deterministic hash, got %s and %s", h1, h2)
		}
		if len(h1) != 64 {
			t.Errorf("expected 64-char hex string, got %d", len(h1))
		}
	})

	t.Run("RandomToken length", func(t *testing.T) {
		tok := utils.RandomToken(16)
		if len(tok) != 32 { // 16 bytes = 32 hex chars
			t.Errorf("expected 32 hex chars, got %d", len(tok))
		}
	})

	t.Run("ConstantTimeCompare", func(t *testing.T) {
		if !utils.ConstantTimeCompare("secret123", "secret123") {
			t.Error("expected equal strings to match")
		}
		if utils.ConstantTimeCompare("secret123", "wrong") {
			t.Error("expected different strings to not match")
		}
	})

	t.Run("Slugify", func(t *testing.T) {
		slug := utils.Slugify("Hello World & Social Media 2026!")
		expected := "hello-world-social-media-2026"
		if slug != expected {
			t.Errorf("expected slug %q, got %q", expected, slug)
		}
	})
}

func TestApiErrors(t *testing.T) {
	err := utils.BadRequest("invalid payload")
	if err.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, err.StatusCode)
	}
	if err.Error() != "invalid payload" {
		t.Errorf("expected message %q, got %q", "invalid payload", err.Error())
	}
}

package utils

import (
	"strings"
)

func NormalizeAccountIDs(raw []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		parts := strings.Split(item, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				if _, exists := seen[trimmed]; !exists {
					seen[trimmed] = struct{}{}
					result = append(result, trimmed)
				}
			}
		}
	}
	return result
}

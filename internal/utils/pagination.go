package utils

import (
	"strconv"
)

type PaginationMeta struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}

func ParsePagination(pageStr, limitStr string) (page, limit, skip int) {
	page = 1
	if p, err := strconv.Atoi(pageStr); err == nil && p >= 1 {
		page = p
	}
	limit = 10
	if l, err := strconv.Atoi(limitStr); err == nil && l >= 1 {
		if l > 100 {
			limit = 100
		} else {
			limit = l
		}
	}
	skip = (page - 1) * limit
	return page, limit, skip
}

package provider

import (
	"net/http"
	"strconv"
)

const (
	DefaultPage    = 1
	DefaultPerPage = 20
	MaxPerPage     = 100
)

// NormalizePageOpts applies default values for page/perPage.
func NormalizePageOpts(page, perPage int) (int, int) {
	if page <= 0 {
		page = DefaultPage
	}
	if perPage <= 0 {
		perPage = DefaultPerPage
	}
	if perPage > MaxPerPage {
		perPage = MaxPerPage
	}
	return page, perPage
}

// ParseTotalCountHeader reads X-Total-Count or X-Total from response headers.
// Falls back to the provided default if neither header is present or valid.
func ParseTotalCountHeader(headers http.Header, fallback int) int {
	for _, key := range []string{"X-Total-Count", "X-Total"} {
		if v := headers.Get(key); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				return n
			}
		}
	}
	return fallback
}

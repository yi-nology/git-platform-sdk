package provider

import (
	"encoding/hex"
	"net/http"
	"strconv"
)

func isCommitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func parseTotalCount(headers http.Header, fallback int) int {
	for _, key := range []string{"X-Total-Count", "X-Total"} {
		if v := headers.Get(key); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				return n
			}
		}
	}
	return fallback
}

package provider

import (
	"encoding/json"
	"net/http"
)

// writeJSON is a tiny test helper that writes v as JSON to w with the right
// Content-Type header. It is intentionally unexported and only available to
// tests in the provider package.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

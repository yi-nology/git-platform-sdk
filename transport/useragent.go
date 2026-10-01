package transport

import (
	"net/http"
	"strings"

	"github.com/yi-nology/go-git-platform/internal/version"
)

// userAgent is the product token this SDK identifies itself with. It is
// appended to (or, when absent, set as) the User-Agent header on every
// outgoing request so platform operators can attribute SDK-driven traffic.
func userAgent() string {
	return "go-git-platform/" + version.String() + " (+https://github.com/yi-nology/go-git-platform)"
}

// setUserAgentDefault sets the SDK's User-Agent token on req. When the
// request already carries a User-Agent (typically the third-party SDK's
// own product token, e.g. go-github), the SDK token is appended to it
// instead of replacing it: both the wrapping SDK and this library then
// appear in the product list, which is valid User-Agent grammar.
func setUserAgentDefault(req *http.Request) {
	ua := userAgent()
	cur := req.Header.Get("User-Agent")
	switch {
	case cur == "":
		req.Header.Set("User-Agent", ua)
	case strings.Contains(cur, "go-git-platform/"):
		// Already identified (e.g. replayed request).
	default:
		req.Header.Set("User-Agent", cur+" "+ua)
	}
}

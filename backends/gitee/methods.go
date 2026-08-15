package gitee

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/yi-nology/git-platform-sdk/transport"
)

// doRequest is a convenience wrapper for JSON-in / JSON-out calls using the
// method/path/body/result signature used throughout the gitee implementation.
func (p *Provider) doRequest(ctx context.Context, method, path string, body, result any) error {
	_, err := p.client.DoJSON(ctx, &transport.Request{
		Method: method,
		Path:   path,
		Body:   body,
		Result: result,
	})
	return err
}

// doRequestWithHeaders is the same as doRequest but returns the response
// headers. Used for paginated endpoints that expose X-Total-Count.
func (p *Provider) doRequestWithHeaders(ctx context.Context, method, path string, body, result any) (http.Header, error) {
	resp, err := p.client.DoJSON(ctx, &transport.Request{
		Method: method,
		Path:   path,
		Body:   body,
		Result: result,
	})
	if err != nil {
		return nil, err
	}
	return resp.Header, nil
}

// esc escapes a single URL path segment. Variable segments interpolated into
// REST paths (owner, repo, branch, label name, sha, ...) must pass through
// this; characters like '#', '?', '%', or spaces would otherwise corrupt or
// truncate the URL.
func esc(s string) string { return url.PathEscape(s) }

// escPath escapes a multi-segment path (e.g. a file path), preserving the
// "/" separators: each segment is percent-encoded, the separators are not.
func escPath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

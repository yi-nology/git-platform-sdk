package gitee

import (
	"context"
	"net/url"
	"strings"

	"github.com/yi-nology/git-platform-sdk/transport"
)

// doRequest is a convenience wrapper for JSON-in / JSON-out calls using the
// method/path/body/result signature used throughout the gitee implementation.
// It serves the surfaces that are still served by the raw transport client;
// SDK-covered surfaces call the generated service methods directly.
func (p *Provider) doRequest(ctx context.Context, method, path string, body, result any) error {
	_, err := p.raw().DoJSON(ctx, &transport.Request{
		Method: method,
		Path:   path,
		Body:   body,
		Result: result,
	})
	return err
}

// esc escapes a single URL path segment. Variable segments interpolated into
// REST paths (owner, repo, branch, label name, sha, ...) must pass through
// this — both for raw-client paths and for SDK calls, since go-gitee
// interpolates path parameters without escaping them. Characters like '#',
// '?', '%', or spaces would otherwise corrupt or truncate the URL.
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

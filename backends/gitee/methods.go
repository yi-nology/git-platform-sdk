package gitee

import (
	"context"
	"net/http"

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

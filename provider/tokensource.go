package provider

import "context"

// TokenSource supplies the access token used to authenticate platform API
// requests. When a Config.TokenSource is set, the transport layer consults
// it on every request, so rotating or expiring credentials — OAuth
// device-flow tokens, GitHub App installation tokens (hourly expiry),
// short-lived CI job tokens — take effect on the next request without
// rebuilding the provider.
//
// The interface is deliberately identical to transport.TokenSource, so any
// implementation satisfies both and backends can forward the value
// unchanged. Implementations must be safe for concurrent use.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// StaticTokenSource returns a TokenSource that always returns token; it is
// the TokenSource equivalent of Config.Token and mostly useful for tests
// and for swapping a static credential into TokenSource-taking code paths.
func StaticTokenSource(token string) TokenSource {
	return staticTokenSource{token: token}
}

type staticTokenSource struct{ token string }

func (s staticTokenSource) Token(context.Context) (string, error) {
	return s.token, nil
}

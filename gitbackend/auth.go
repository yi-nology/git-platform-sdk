package gitbackend

// NewTokenAuth builds an AuthConfig for HTTPS token authentication.
//
// When authenticating against git hosting platforms (GitHub, GitLab, Gitea,
// etc.) over HTTPS the credentials are exchanged via HTTP Basic Auth: the
// username is an arbitrary non-empty placeholder and the real token is sent as
// the password. An empty token collapses to AuthNone so callers can simply pass
// through an optional token without extra branching.
func NewTokenAuth(token string) AuthConfig {
	if token == "" {
		return AuthConfig{Type: AuthNone}
	}
	return AuthConfig{
		Type:     AuthHTTPBasic,
		Username: "token",
		Password: token,
	}
}

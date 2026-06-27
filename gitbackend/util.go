package gitbackend

// isCommitSHA reports whether s looks like a full 40-character git object SHA.
// It is shared by both backends.
func isCommitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// mergeInsecure returns a copy of auth with InsecureSkipTLS set when either the
// AuthConfig itself or the explicit per-call flag requests skipping TLS
// verification. Lets Fetch/Push/Clone honor both opt paths (opts.InsecureSkipTLS
// and opts.Auth.InsecureSkipTLS) in one place.
func mergeInsecure(auth AuthConfig, insecure bool) AuthConfig {
	if insecure {
		auth.InsecureSkipTLS = true
	}
	return auth
}

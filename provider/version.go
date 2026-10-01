package provider

import "github.com/yi-nology/go-git-platform/internal/version"

// Version returns the runtime version of the go-git-platform module, as
// resolved from Go build info. It reports "dev" for development builds
// (in-module go run/go test) where no module version is stamped.
//
// The value is also injected as part of the default User-Agent header on
// every outgoing HTTP request made through the transport layer, so platform
// operators can identify (and, where offered, rate-limit-mercy) SDK-driven
// traffic.
func Version() string { return version.String() }

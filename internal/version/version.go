// Package version resolves the runtime version of the go-git-platform
// module. It is the single source of truth behind provider.Version() and
// the default User-Agent injected by the transport layer.
//
// When the module is compiled as a dependency, debug.ReadBuildInfo reports
// the resolved (pseudo-)version of this module among the main module's
// dependencies — including replaces. When the module itself is the main
// module (go test, local go run/build), Go reports "(devel)" or an empty
// version and the fallback "dev" is returned.
package version

import (
	"runtime/debug"
	"strings"
)

// ModulePath is this module's Go module path.
const ModulePath = "github.com/yi-nology/go-git-platform"

// fallbackVersion is reported when no version can be recovered from build
// info (development builds, tests).
const fallbackVersion = "dev"

// String returns the module version, or "dev" when unavailable.
func String() string {
	if v := fromBuildInfo(); v != "" {
		return v
	}
	return fallbackVersion
}

func fromBuildInfo() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	// The module itself is the main module (tests, local builds). Newer Go
	// toolchains tag main-module versions from VCS; "(devel)" means unknown.
	if bi.Main.Path == ModulePath || strings.HasPrefix(bi.Main.Path, ModulePath+"/") {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	for _, dep := range bi.Deps {
		if dep.Path != ModulePath {
			continue
		}
		if v := dep.Version; v != "" {
			return v
		}
		// Filesystem replace (e.g. the mcp submodule): the version, if any,
		// lives on the replace target.
		if dep.Replace != nil {
			if v := dep.Replace.Version; v != "" {
				return v
			}
		}
	}
	return ""
}

// Package all imports every platform backend that ships with the SDK. Import
// it with a blank identifier once at the start of your program to register
// all platforms with provider:
//
//	import _ "github.com/yi-nology/git-platform-sdk/backends/all"
//
// After that, provider.NewProvider accepts any platform configured via
// provider.Config.
package all

import (
	// Each backend self-registers via its own init().
	_ "github.com/yi-nology/git-platform-sdk/backends/forgejo"
	_ "github.com/yi-nology/git-platform-sdk/backends/gitea"
	_ "github.com/yi-nology/git-platform-sdk/backends/github"
	_ "github.com/yi-nology/git-platform-sdk/backends/gitcode"
	_ "github.com/yi-nology/git-platform-sdk/backends/gitee"
	_ "github.com/yi-nology/git-platform-sdk/backends/gitlab"
	_ "github.com/yi-nology/git-platform-sdk/backends/tencentcode"
)
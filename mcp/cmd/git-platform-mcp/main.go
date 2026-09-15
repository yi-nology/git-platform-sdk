// Command git-platform-mcp runs a Model Context Protocol server over
// git-platform-sdk, exposing one tool surface for GitHub, GitLab, Gitea,
// Forgejo, Gitee, GitCode, and Tencent Code.
//
// Usage:
//
//	git-platform-mcp --platform gitea --base-url https://gitea.example.com --token-env GITEA_TOKEN
//
// Flags:
//
//	--platform    one of: github, gitlab, gitea, forgejo, gitee, gitcode, tencentcode
//	--base-url    instance base URL; empty uses the platform's public host
//	--token       API token (prefer --token-env in shells; stdin tokens are
//	              also accepted via GIT_PLATFORM_TOKEN when the flag is unset)
//	--token-env   name of the environment variable holding the token
//	--read-only   mount only read tools (recommended for untrusted agents)
//	--toolsets    comma-separated subset: core,crs,issues,status,search
//
// The server speaks MCP over stdio, so a typical client config is:
//
//	{"command": "git-platform-mcp", "args": ["--platform", "gitea", "--token-env", "GITEA_TOKEN"]}
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcpserver "github.com/yi-nology/git-platform-sdk/mcp"
	"github.com/yi-nology/git-platform-sdk/provider"

	// register every shipped backend
	_ "github.com/yi-nology/git-platform-sdk/backends/all"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var (
		platform = flag.String("platform", os.Getenv("GIT_PLATFORM"), "github|gitlab|gitea|forgejo|gitee|gitcode|tencentcode")
		baseURL  = flag.String("base-url", os.Getenv("GIT_PLATFORM_URL"), "instance base URL (empty = public host)")
		token    = flag.String("token", "", "API token (prefer --token-env)")
		tokenEnv = flag.String("token-env", "GIT_PLATFORM_TOKEN", "environment variable holding the API token")
		skipTLS  = flag.Bool("skip-tls", false, "skip TLS verification for self-hosted instances (not recommended)")
		readOnly = flag.Bool("read-only", false, "mount read tools only")
		toolsets = flag.String("toolsets", "", "comma-separated subset of core,crs,issues,status,search (empty = all)")
	)
	flag.Parse()

	if *platform == "" {
		return fmt.Errorf("--platform (or GIT_PLATFORM) is required: github, gitlab, gitea, forgejo, gitee, gitcode, tencentcode")
	}
	tok := *token
	if tok == "" {
		tok = os.Getenv(*tokenEnv)
	}
	if tok == "" {
		return fmt.Errorf("no token: pass --token or set %s", *tokenEnv)
	}

	cfg := provider.Config{
		Platform: provider.Platform(*platform),
		BaseURL:  *baseURL,
		Token:    tok,
		SkipTLS:  *skipTLS,
	}
	p, err := provider.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	opts := mcpserver.Options{ReadOnly: *readOnly}
	if *toolsets != "" {
		for _, t := range strings.Split(*toolsets, ",") {
			if t = strings.TrimSpace(t); t != "" {
				opts.Toolsets = append(opts.Toolsets, t)
			}
		}
	}

	srv := mcpserver.NewServer(p, opts)

	ctx := context.Background()
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("mcp server: %w", err)
	}
	return nil
}

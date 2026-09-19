// Package mcpserver exposes git-platform-sdk as a Model Context Protocol
// (MCP) server so AI agents can operate GitHub, GitLab, Gitea, Forgejo,
// Gitee, GitCode, and Tencent Code through one tool surface.
//
// Design conventions mirror the platform MCP servers agents already know:
//
//   - Toolsets group tools by domain (core, crs, issues, status, search)
//     and can be selected at construction time.
//   - Read/write separation: every mutating tool carries a
//     "mutating: ..." annotation; Options.ReadOnly drops them all.
//   - Capability gating: a tool is only registered when the connected
//     provider's Capabilities() declares the underlying capability, so
//     the model never sees tools that would fail with
//     ErrNotImplemented.
package mcpserver

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// Version is the MCP server's own version.
const Version = "0.1.0"

// Options configures NewServer. A zero Options is valid: all toolsets
// are enabled, read/write mode is on.
type Options struct {
	// Toolsets selects which toolsets to mount. Empty or nil mounts all.
	// Unknown names make NewServer return an error.
	Toolsets []string
	// ReadOnly drops every mutating tool at registration time: the model
	// never sees (and cannot call) the mutations.
	ReadOnly bool
	// Name/Version reported in the MCP initialize handshake; defaults
	// to "git-platform-sdk" / the module version.
	Name string
}

// toolset is a named group of tools sharing one capability gate.
type toolset struct {
	name       string
	enabled    func(caps provider.CapabilitySet) bool
	registered func(s *mcp.Server, st *state)
}

// state carries everything the tool handlers need.
type state struct {
	p        provider.Provider
	readonly bool
}

// NewServer builds an MCP server over the given provider. The provider
// must already be constructed and authenticated (see the cmd package for
// the flag/environment wiring). Unknown names in opts.Toolsets are an
// error: a misspelled toolset would otherwise silently shrink the tool
// surface the model sees.
func NewServer(p provider.Provider, opts Options) (*mcp.Server, error) {
	name := opts.Name
	if name == "" {
		name = "git-platform-sdk"
	}
	s := mcp.NewServer(&mcp.Implementation{Name: name, Version: Version}, nil)

	st := &state{p: p, readonly: opts.ReadOnly}
	caps := p.Capabilities()
	selected := map[string]bool{}
	for _, t := range opts.Toolsets {
		if !knownToolsets[t] {
			return nil, fmt.Errorf("unknown toolset %q (known: core, crs, issues, status, search)", t)
		}
		selected[t] = true
	}
	want := func(set string) bool { return len(selected) == 0 || selected[set] }

	for _, ts := range toolsets {
		if !want(ts.name) || !ts.enabled(caps) {
			continue
		}
		ts.registered(s, st)
	}
	return s, nil
}

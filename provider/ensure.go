package provider

import (
	"context"
	"slices"
)

// EnsureAction reports what an Ensure* call did to converge the resource
// onto its desired state.
type EnsureAction string

// Ensure* outcomes.
const (
	// EnsureCreated: the resource did not exist and was created.
	EnsureCreated EnsureAction = "created"
	// EnsureUpdated: the resource existed with different content and was
	// updated (for webhooks and deploy keys: replaced, see below).
	EnsureUpdated EnsureAction = "updated"
	// EnsureUnchanged: the resource already matched the desired state.
	EnsureUnchanged EnsureAction = "unchanged"
)

// EnsureWebhook makes the repository carry exactly one webhook for
// opts.URL with opts.Events, creating or replacing as needed, and returns
// the resulting webhook plus the action taken. It is the idempotent
// building block automation wants: "make this hook exist, whatever it
// takes", safe to re-run.
//
// Semantics:
//
//   - No webhook with that URL → created.
//   - Same URL, same event set → unchanged. Secrets are write-only on
//     every platform (list responses never return them), so an existing
//     hook's secret is not comparable; re-running with a different secret
//     reports unchanged and keeps the old secret.
//   - Same URL, different event set → updated. The WebhookManager
//     interface has no update method, so the hook is deleted and
//     recreated; its platform ID changes as a result.
//   - Empty opts.Events defaults to {push, pull_request}, matching
//     CreateWebhook's platform-side default.
//
// Multiple existing hooks with the same URL are all replaced by one.
func EnsureWebhook(ctx context.Context, p WebhookManager, opts CreateWebhookOptions) (EnsureAction, *PlatformWebhook, error) {
	events := opts.Events
	if len(events) == 0 {
		events = []string{"push", "pull_request"}
	}
	desired := slices.Clone(events)
	slices.Sort(desired)

	existing, err := p.ListWebhooks(ctx, opts.Owner, opts.Repo)
	if err != nil {
		return "", nil, err
	}

	var matches []*PlatformWebhook
	for _, h := range existing {
		if h != nil && h.URL == opts.URL {
			matches = append(matches, h)
		}
	}

	if len(matches) == 1 && equalEvents(matches[0].Events, desired) {
		return EnsureUnchanged, matches[0], nil
	}

	// Drift (or duplicates): replace every matching hook with one fresh one.
	for _, h := range matches {
		if err := p.DeleteWebhook(ctx, opts.Owner, opts.Repo, h.ID); err != nil {
			return "", nil, err
		}
	}
	hook, err := p.CreateWebhook(ctx, CreateWebhookOptions{
		Owner: opts.Owner, Repo: opts.Repo, URL: opts.URL,
		Secret: opts.Secret, Events: events,
	})
	if err != nil {
		return "", nil, err
	}
	if len(matches) == 0 {
		return EnsureCreated, hook, nil
	}
	return EnsureUpdated, hook, nil
}

func equalEvents(a []string, sortedB []string) bool {
	sa := slices.Clone(a)
	slices.Sort(sa)
	return slices.Equal(sa, sortedB)
}

// EnsureBranchProtection converges the branch protection rule for
// opts.BranchName onto opts, creating, updating, or leaving it untouched
// as required, and returns the resulting rule plus the action taken.
// It targets the BranchProtectionManager directly so it works with any
// Provider (type-assert it at the call site) or test double.
func EnsureBranchProtection(ctx context.Context, bpm BranchProtectionManager, owner, repo string, opts CreateBranchProtectionOptions) (EnsureAction, *BranchProtection, error) {
	cur, err := bpm.GetBranchProtection(ctx, owner, repo, opts.BranchName)
	if err == nil {
		if cur != nil &&
			cur.RequiredApprovingReviews == opts.RequiredApprovingReviews &&
			cur.RequiredStatusChecks == opts.RequiredStatusChecks &&
			cur.AllowForcePushes == opts.AllowForcePushes &&
			cur.AllowDeletions == opts.AllowDeletions {
			return EnsureUnchanged, cur, nil
		}
		out, err := bpm.UpdateBranchProtection(ctx, owner, repo, opts.BranchName, UpdateBranchProtectionOptions{
			RequiredApprovingReviews: &opts.RequiredApprovingReviews,
			RequiredStatusChecks:     &opts.RequiredStatusChecks,
			AllowForcePushes:         &opts.AllowForcePushes,
			AllowDeletions:           &opts.AllowDeletions,
		})
		if err != nil {
			return "", nil, err
		}
		return EnsureUpdated, out, nil
	}
	if !IsNotFound(err) {
		return "", nil, err
	}
	out, err := bpm.CreateBranchProtection(ctx, owner, repo, opts)
	if err != nil {
		return "", nil, err
	}
	return EnsureCreated, out, nil
}

// EnsureDeployKey converges the repository's deploy key titled
// opts.Title onto opts, creating, replacing, or leaving it untouched as
// required, and returns the resulting key plus the action taken.
//
// Existing keys are matched by title; a title collision with different
// key material or read-only flag replaces the key (the manager interface
// has no update: delete + re-add), which changes its platform ID.
func EnsureDeployKey(ctx context.Context, dkm DeploymentKeyManager, owner, repo string, opts AddDeployKeyOptions) (EnsureAction, *DeployKey, error) {
	existing, err := dkm.ListDeployKeys(ctx, owner, repo)
	if err != nil {
		return "", nil, err
	}
	for _, k := range existing {
		if k == nil || k.Title != opts.Title {
			continue
		}
		if k.Key == opts.Key && k.ReadOnly == opts.ReadOnly {
			return EnsureUnchanged, k, nil
		}
		if err := dkm.DeleteDeployKey(ctx, owner, repo, k.ID); err != nil {
			return "", nil, err
		}
		out, err := dkm.AddDeployKey(ctx, owner, repo, opts)
		if err != nil {
			return "", nil, err
		}
		return EnsureUpdated, out, nil
	}
	out, err := dkm.AddDeployKey(ctx, owner, repo, opts)
	if err != nil {
		return "", nil, err
	}
	return EnsureCreated, out, nil
}

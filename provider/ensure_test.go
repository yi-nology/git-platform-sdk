package provider

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// --- fakes ---

type fakeWebhookManager struct {
	hooks   []*PlatformWebhook
	nextID  int64
	created CreateWebhookOptions
}

func (f *fakeWebhookManager) ListWebhooks(context.Context, string, string) ([]*PlatformWebhook, error) {
	return f.hooks, nil
}

func (f *fakeWebhookManager) CreateWebhook(_ context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error) {
	f.nextID++
	events := opts.Events
	if len(events) == 0 {
		events = []string{"push", "pull_request"}
	}
	f.created = opts
	h := &PlatformWebhook{ID: f.nextID, URL: opts.URL, Events: events}
	f.hooks = append(f.hooks, h)
	return h, nil
}

func (f *fakeWebhookManager) DeleteWebhook(_ context.Context, _, _ string, id int64) error {
	for i, h := range f.hooks {
		if h.ID == id {
			f.hooks = append(f.hooks[:i], f.hooks[i+1:]...)
			return nil
		}
	}
	return nil
}

func (f *fakeWebhookManager) ParseWebhookEvent(*http.Request, string) (*NormalizedEvent, error) {
	return nil, ErrNotImplemented
}

func (f *fakeWebhookManager) ValidateWebhookSignature(*http.Request, string) error { return nil }

type fakeBranchProtection struct {
	rules    map[string]*BranchProtection
	getFails error
}

func (f *fakeBranchProtection) ListBranchProtections(context.Context, string, string) ([]*BranchProtection, error) {
	out := make([]*BranchProtection, 0, len(f.rules))
	for _, r := range f.rules {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeBranchProtection) GetBranchProtection(_ context.Context, _, _, branch string) (*BranchProtection, error) {
	if f.getFails != nil {
		return nil, f.getFails
	}
	r, ok := f.rules[branch]
	if !ok {
		return nil, &ProviderError{Op: "GetBranchProtection", Cause: ErrNotFound}
	}
	return r, nil
}

func (f *fakeBranchProtection) CreateBranchProtection(_ context.Context, _, _ string, opts CreateBranchProtectionOptions) (*BranchProtection, error) {
	r := &BranchProtection{
		BranchName: opts.BranchName, RequiredApprovingReviews: opts.RequiredApprovingReviews,
		RequiredStatusChecks: opts.RequiredStatusChecks, AllowForcePushes: opts.AllowForcePushes,
		AllowDeletions: opts.AllowDeletions,
	}
	f.rules[opts.BranchName] = r
	return r, nil
}

func (f *fakeBranchProtection) UpdateBranchProtection(_ context.Context, _, _, branch string, opts UpdateBranchProtectionOptions) (*BranchProtection, error) {
	r := f.rules[branch]
	if opts.RequiredApprovingReviews != nil {
		r.RequiredApprovingReviews = *opts.RequiredApprovingReviews
	}
	if opts.RequiredStatusChecks != nil {
		r.RequiredStatusChecks = *opts.RequiredStatusChecks
	}
	if opts.AllowForcePushes != nil {
		r.AllowForcePushes = *opts.AllowForcePushes
	}
	if opts.AllowDeletions != nil {
		r.AllowDeletions = *opts.AllowDeletions
	}
	return r, nil
}

func (f *fakeBranchProtection) DeleteBranchProtection(_ context.Context, _, _, branch string) error {
	delete(f.rules, branch)
	return nil
}

type fakeDeployKeys struct {
	keys  []*DeployKey
	next  int64
	added AddDeployKeyOptions
}

func (f *fakeDeployKeys) ListDeployKeys(context.Context, string, string) ([]*DeployKey, error) {
	return f.keys, nil
}

func (f *fakeDeployKeys) AddDeployKey(_ context.Context, _, _ string, opts AddDeployKeyOptions) (*DeployKey, error) {
	f.next++
	f.added = opts
	k := &DeployKey{ID: f.next, Title: opts.Title, Key: opts.Key, ReadOnly: opts.ReadOnly}
	f.keys = append(f.keys, k)
	return k, nil
}

func (f *fakeDeployKeys) DeleteDeployKey(_ context.Context, _, _ string, id int64) error {
	for i, k := range f.keys {
		if k.ID == id {
			f.keys = append(f.keys[:i], f.keys[i+1:]...)
			return nil
		}
	}
	return nil
}

// --- webhooks ---

func TestEnsureWebhookCreates(t *testing.T) {
	f := &fakeWebhookManager{}
	action, hook, err := EnsureWebhook(t.Context(), f, CreateWebhookOptions{Owner: "o", Repo: "r", URL: "https://cb.example/hook"})
	if err != nil {
		t.Fatalf("EnsureWebhook: %v", err)
	}
	if action != EnsureCreated {
		t.Fatalf("action = %q, want created", action)
	}
	if hook.ID != 1 || len(f.hooks) != 1 {
		t.Fatalf("hook = %+v, hooks = %+v", hook, f.hooks)
	}
	// Default events applied.
	if len(f.created.Events) != 2 {
		t.Fatalf("created events = %v", f.created.Events)
	}
}

func TestEnsureWebhookUnchangedOnReRun(t *testing.T) {
	f := &fakeWebhookManager{}
	_, _, err := EnsureWebhook(t.Context(), f, CreateWebhookOptions{Owner: "o", Repo: "r", URL: "https://cb.example/hook", Events: []string{"push", "pull_request"}})
	if err != nil {
		t.Fatal(err)
	}
	before := f.hooks[0].ID
	action, hook, err := EnsureWebhook(t.Context(), f, CreateWebhookOptions{Owner: "o", Repo: "r", URL: "https://cb.example/hook"})
	if err != nil {
		t.Fatal(err)
	}
	if action != EnsureUnchanged || hook.ID != before {
		t.Fatalf("action = %q id = %d, want unchanged/%d", action, hook.ID, before)
	}
}

func TestEnsureWebhookReplacesOnEventDrift(t *testing.T) {
	f := &fakeWebhookManager{}
	if _, _, err := EnsureWebhook(t.Context(), f, CreateWebhookOptions{Owner: "o", Repo: "r", URL: "https://cb.example/hook"}); err != nil {
		t.Fatal(err)
	}
	f.hooks[0].Events = []string{"push"} // drift
	action, hook, err := EnsureWebhook(t.Context(), f, CreateWebhookOptions{Owner: "o", Repo: "r", URL: "https://cb.example/hook"})
	if err != nil {
		t.Fatal(err)
	}
	if action != EnsureUpdated {
		t.Fatalf("action = %q, want updated", action)
	}
	if len(f.hooks) != 1 || hook.ID != 2 {
		t.Fatalf("hooks = %+v, want single replaced hook with new ID", f.hooks)
	}
}

// --- branch protection ---

func TestEnsureBranchProtectionLifecycle(t *testing.T) {
	f := &fakeBranchProtection{rules: map[string]*BranchProtection{}}
	opts := CreateBranchProtectionOptions{BranchName: "main", RequiredApprovingReviews: 2, RequiredStatusChecks: true}

	action, _, err := EnsureBranchProtection(t.Context(), f, "o", "r", opts)
	if err != nil || action != EnsureCreated {
		t.Fatalf("create: action=%q err=%v", action, err)
	}
	action, _, err = EnsureBranchProtection(t.Context(), f, "o", "r", opts)
	if err != nil || action != EnsureUnchanged {
		t.Fatalf("re-run: action=%q err=%v", action, err)
	}
	opts.RequiredApprovingReviews = 1
	action, rule, err := EnsureBranchProtection(t.Context(), f, "o", "r", opts)
	if err != nil || action != EnsureUpdated {
		t.Fatalf("update: action=%q err=%v", action, err)
	}
	if rule.RequiredApprovingReviews != 1 {
		t.Fatalf("rule = %+v, want reviews=1", rule)
	}
}

func TestEnsureBranchProtectionPropagatesRealErrors(t *testing.T) {
	boom := errors.New("boom")
	f := &fakeBranchProtection{getFails: boom}
	_, _, err := EnsureBranchProtection(t.Context(), f, "o", "r", CreateBranchProtectionOptions{BranchName: "main"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

// --- deploy keys ---

func TestEnsureDeployKeyLifecycle(t *testing.T) {
	f := &fakeDeployKeys{}
	opts := AddDeployKeyOptions{Title: "ci", Key: "ssh-ed25519 AAA", ReadOnly: true}

	action, _, err := EnsureDeployKey(t.Context(), f, "o", "r", opts)
	if err != nil || action != EnsureCreated {
		t.Fatalf("create: action=%q err=%v", action, err)
	}
	action, _, err = EnsureDeployKey(t.Context(), f, "o", "r", opts)
	if err != nil || action != EnsureUnchanged {
		t.Fatalf("re-run: action=%q err=%v", action, err)
	}
	opts.Key = "ssh-ed25519 BBB"
	action, key, err := EnsureDeployKey(t.Context(), f, "o", "r", opts)
	if err != nil || action != EnsureUpdated {
		t.Fatalf("replace: action=%q err=%v", action, err)
	}
	if len(f.keys) != 1 || key.Key != "ssh-ed25519 BBB" {
		t.Fatalf("keys = %+v", f.keys)
	}
}

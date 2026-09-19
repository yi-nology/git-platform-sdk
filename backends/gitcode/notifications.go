package gitcode

import (
	"context"
	"strconv"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
	gitcode "github.com/yi-nology/go-gitcode"
)

// ListNotifications implements provider.NotificationManager.
//
// Dual-mode pagination: opts.Page == 0 fetches every page via AllPages
// (GitCode's page-size ceiling is 100); opts.Page > 0 returns exactly that
// single page and the caller drives pagination itself.
func (p *Provider) ListNotifications(ctx context.Context, opts provider.ListNotificationsOptions) ([]*provider.Notification, error) {
	buildOpts := func(page, perPage int) gitcode.ListNotificationsOptions {
		return gitcode.ListNotificationsOptions{
			ListOptions: gitcode.ListOptions{Page: page, PerPage: perPage},
			All:         opts.All,
			Since:       opts.Since,
		}
	}
	var threads []*gitcode.NotificationThread
	if opts.Page > 0 {
		// Caller-driven pagination: serve the requested page only.
		perPage := opts.PerPage
		if perPage <= 0 || perPage > provider.MaxPerPage {
			perPage = provider.MaxPerPage
		}
		var err error
		if threads, err = p.client.ListNotificationsWithOptions(ctx, buildOpts(opts.Page, perPage)); err != nil {
			return nil, provider.Wrap(provider.PlatformGitCode, "ListNotifications", err)
		}
	} else {
		var err error
		if threads, err = backendutil.AllPages(func(page int) ([]*gitcode.NotificationThread, error) {
			return p.client.ListNotificationsWithOptions(ctx, buildOpts(page, provider.MaxPerPage))
		}); err != nil {
			return nil, provider.Wrap(provider.PlatformGitCode, "ListNotifications", err)
		}
	}
	result := make([]*provider.Notification, 0, len(threads))
	for _, t := range threads {
		result = append(result, convertNotification(t))
	}
	return result, nil
}

// ListRepoNotifications implements provider.NotificationManager.
//
// Dual-mode pagination, mirroring ListNotifications: opts.Page == 0 fetches
// every page via AllPages; opts.Page > 0 returns exactly that single page.
func (p *Provider) ListRepoNotifications(ctx context.Context, owner, repo string, opts provider.ListNotificationsOptions) ([]*provider.Notification, error) {
	buildOpts := func(page, perPage int) gitcode.ListNotificationsOptions {
		return gitcode.ListNotificationsOptions{
			ListOptions: gitcode.ListOptions{Page: page, PerPage: perPage},
			All:         opts.All,
			Since:       opts.Since,
		}
	}
	var threads []*gitcode.NotificationThread
	if opts.Page > 0 {
		// Caller-driven pagination: serve the requested page only.
		perPage := opts.PerPage
		if perPage <= 0 || perPage > provider.MaxPerPage {
			perPage = provider.MaxPerPage
		}
		var err error
		if threads, err = p.client.ListRepoNotifications(ctx, owner, repo, buildOpts(opts.Page, perPage)); err != nil {
			return nil, provider.Wrap(provider.PlatformGitCode, "ListRepoNotifications", err)
		}
	} else {
		var err error
		if threads, err = backendutil.AllPages(func(page int) ([]*gitcode.NotificationThread, error) {
			return p.client.ListRepoNotifications(ctx, owner, repo, buildOpts(page, provider.MaxPerPage))
		}); err != nil {
			return nil, provider.Wrap(provider.PlatformGitCode, "ListRepoNotifications", err)
		}
	}
	result := make([]*provider.Notification, 0, len(threads))
	for _, t := range threads {
		result = append(result, convertNotification(t))
	}
	return result, nil
}

// MarkNotificationRead implements provider.NotificationManager.
func (p *Provider) MarkNotificationRead(ctx context.Context, threadID string) error {
	id, err := strconv.ParseInt(threadID, 10, 64)
	if err != nil {
		return provider.Wrapf(provider.PlatformGitCode, "MarkNotificationRead", "invalid thread ID %q", threadID)
	}
	return provider.Wrap(provider.PlatformGitCode, "MarkNotificationRead", p.client.MarkNotificationThreadAsRead(ctx, id))
}

// MarkNotificationsRead implements provider.NotificationManager.
func (p *Provider) MarkNotificationsRead(ctx context.Context, opts provider.MarkNotificationsOptions) error {
	return provider.Wrap(provider.PlatformGitCode, "MarkNotificationsRead", p.client.MarkNotificationsAsRead(ctx, gitcode.MarkNotificationsOptions{
		LastReadAt: opts.LastReadAt,
	}))
}

// MarkRepoNotificationsRead implements provider.NotificationManager.
func (p *Provider) MarkRepoNotificationsRead(ctx context.Context, owner, repo string, opts provider.MarkNotificationsOptions) error {
	return provider.Wrap(provider.PlatformGitCode, "MarkRepoNotificationsRead", p.client.MarkRepoNotificationsAsRead(ctx, owner, repo, gitcode.MarkNotificationsOptions{
		LastReadAt: opts.LastReadAt,
	}))
}

var _ provider.NotificationManager = (*Provider)(nil)

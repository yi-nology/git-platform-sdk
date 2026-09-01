package gitcode

import (
	"context"
	"strconv"

	"github.com/yi-nology/git-platform-sdk/provider"
	gitcode "github.com/yi-nology/go-gitcode"
)

// ListNotifications implements provider.NotificationManager.
func (p *Provider) ListNotifications(ctx context.Context, opts provider.ListNotificationsOptions) ([]*provider.Notification, error) {
	listOpts := gitcode.ListNotificationsOptions{
		All:   opts.All,
		Since: opts.Since,
	}
	threads, err := p.client.ListNotificationsWithOptions(ctx, listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListNotifications", err)
	}
	result := make([]*provider.Notification, 0, len(threads))
	for _, t := range threads {
		result = append(result, convertNotification(t))
	}
	return result, nil
}

// ListRepoNotifications implements provider.NotificationManager.
func (p *Provider) ListRepoNotifications(ctx context.Context, owner, repo string, opts provider.ListNotificationsOptions) ([]*provider.Notification, error) {
	listOpts := gitcode.ListNotificationsOptions{
		All:   opts.All,
		Since: opts.Since,
	}
	threads, err := p.client.ListRepoNotifications(ctx, owner, repo, listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListRepoNotifications", err)
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

func convertNotification(t *gitcode.NotificationThread) *provider.Notification {
	n := &provider.Notification{
		ID:     strconv.FormatInt(t.ID, 10),
		Unread: t.Unread,
	}
	if t.Subject != nil {
		n.Subject = provider.NotificationSubject{
			Title: t.Subject.Title,
			Type:  t.Subject.Type,
			URL:   t.Subject.URL,
		}
	}
	if t.Repository != nil {
		n.Repo = &provider.EventRepo{
			ID:       t.Repository.ID,
			FullName: t.Repository.FullName,
		}
		owner, name := provider.SplitFullName(t.Repository.FullName)
		n.Repo.Owner = owner
		n.Repo.Name = name
	}
	n.UpdatedAt = t.UpdatedAt
	return n
}

var _ provider.NotificationManager = (*Provider)(nil)

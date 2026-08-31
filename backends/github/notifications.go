package github

import (
	"context"
	"time"

	"github.com/google/go-github/v72/github"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListNotifications implements provider.NotificationManager.
func (p *Provider) ListNotifications(ctx context.Context, opts provider.ListNotificationsOptions) ([]*provider.Notification, error) {
	listOpts := &github.NotificationListOptions{
		All: opts.All,
	}
	if opts.Since != "" {
		if t, err := time.Parse(time.RFC3339, opts.Since); err == nil {
			listOpts.Since = t
		}
	}
	threads, _, err := p.client.Activity.ListNotifications(ctx, listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListNotifications", err)
	}
	result := make([]*provider.Notification, 0, len(threads))
	for _, t := range threads {
		result = append(result, convertNotification(t))
	}
	return result, nil
}

// ListRepoNotifications implements provider.NotificationManager.
func (p *Provider) ListRepoNotifications(ctx context.Context, owner, repo string, opts provider.ListNotificationsOptions) ([]*provider.Notification, error) {
	listOpts := &github.NotificationListOptions{
		All: opts.All,
	}
	if opts.Since != "" {
		if t, err := time.Parse(time.RFC3339, opts.Since); err == nil {
			listOpts.Since = t
		}
	}
	threads, _, err := p.client.Activity.ListRepositoryNotifications(ctx, owner, repo, listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListRepoNotifications", err)
	}
	result := make([]*provider.Notification, 0, len(threads))
	for _, t := range threads {
		result = append(result, convertNotification(t))
	}
	return result, nil
}

// MarkNotificationRead implements provider.NotificationManager.
func (p *Provider) MarkNotificationRead(ctx context.Context, threadID string) error {
	_, err := p.client.Activity.MarkThreadRead(ctx, threadID)
	return provider.Wrap(provider.PlatformGitHub, "MarkNotificationRead", err)
}

// MarkNotificationsRead implements provider.NotificationManager.
func (p *Provider) MarkNotificationsRead(ctx context.Context, opts provider.MarkNotificationsOptions) error {
	var lastRead github.Timestamp
	if opts.LastReadAt != "" {
		if t, err := time.Parse(time.RFC3339, opts.LastReadAt); err == nil {
			lastRead = github.Timestamp{Time: t}
		}
	}
	_, err := p.client.Activity.MarkNotificationsRead(ctx, lastRead)
	return provider.Wrap(provider.PlatformGitHub, "MarkNotificationsRead", err)
}

// MarkRepoNotificationsRead implements provider.NotificationManager.
func (p *Provider) MarkRepoNotificationsRead(ctx context.Context, owner, repo string, opts provider.MarkNotificationsOptions) error {
	var lastRead github.Timestamp
	if opts.LastReadAt != "" {
		if t, err := time.Parse(time.RFC3339, opts.LastReadAt); err == nil {
			lastRead = github.Timestamp{Time: t}
		}
	}
	_, err := p.client.Activity.MarkRepositoryNotificationsRead(ctx, owner, repo, lastRead)
	return provider.Wrap(provider.PlatformGitHub, "MarkRepoNotificationsRead", err)
}

func convertNotification(n *github.Notification) *provider.Notification {
	out := &provider.Notification{
		ID:     n.GetID(),
		Unread: n.GetUnread(),
		Reason: n.GetReason(),
	}
	if n.Subject != nil {
		out.Subject = provider.NotificationSubject{
			Title: n.Subject.GetTitle(),
			Type:  n.Subject.GetType(),
			URL:   n.Subject.GetURL(),
		}
	}
	if n.Repository != nil {
		out.Repo = &provider.EventRepo{
			ID:       n.Repository.GetID(),
			FullName: n.Repository.GetFullName(),
		}
		owner, name := provider.SplitFullName(n.Repository.GetFullName())
		out.Repo.Owner = owner
		out.Repo.Name = name
	}
	if n.UpdatedAt != nil {
		out.UpdatedAt = n.UpdatedAt.Time
	}
	return out
}

var _ provider.NotificationManager = (*Provider)(nil)

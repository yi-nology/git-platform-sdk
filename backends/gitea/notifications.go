package gitea

import (
	"context"
	"strconv"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListNotifications implements provider.NotificationManager.
func (p *Provider) ListNotifications(ctx context.Context, opts provider.ListNotificationsOptions) ([]*provider.Notification, error) {
	listOpts := gitea.ListNotificationOptions{}
	if opts.Since != "" {
		listOpts.Since, _ = parseTime(opts.Since)
	}
	if opts.All {
		listOpts.Status = []gitea.NotifyStatus{gitea.NotifyStatusRead, gitea.NotifyStatusUnread, gitea.NotifyStatusPinned}
	}
	threads, _, err := p.client.ListNotifications(listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListNotifications", err)
	}
	result := make([]*provider.Notification, 0, len(threads))
	for _, t := range threads {
		result = append(result, convertNotification(t))
	}
	return result, nil
}

// ListRepoNotifications implements provider.NotificationManager.
func (p *Provider) ListRepoNotifications(ctx context.Context, owner, repo string, opts provider.ListNotificationsOptions) ([]*provider.Notification, error) {
	listOpts := gitea.ListNotificationOptions{}
	if opts.Since != "" {
		listOpts.Since, _ = parseTime(opts.Since)
	}
	if opts.All {
		listOpts.Status = []gitea.NotifyStatus{gitea.NotifyStatusRead, gitea.NotifyStatusUnread, gitea.NotifyStatusPinned}
	}
	threads, _, err := p.client.ListRepoNotifications(owner, repo, listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListRepoNotifications", err)
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
		return provider.Wrapf(provider.PlatformGitea, "MarkNotificationRead", "invalid thread ID %q", threadID)
	}
	_, _, err = p.client.ReadNotification(id)
	return provider.Wrap(provider.PlatformGitea, "MarkNotificationRead", err)
}

// MarkNotificationsRead implements provider.NotificationManager.
func (p *Provider) MarkNotificationsRead(ctx context.Context, opts provider.MarkNotificationsOptions) error {
	markOpts := gitea.MarkNotificationOptions{}
	if opts.LastReadAt != "" {
		markOpts.LastReadAt, _ = parseTime(opts.LastReadAt)
	}
	_, _, err := p.client.ReadNotifications(markOpts)
	return provider.Wrap(provider.PlatformGitea, "MarkNotificationsRead", err)
}

// MarkRepoNotificationsRead implements provider.NotificationManager.
func (p *Provider) MarkRepoNotificationsRead(ctx context.Context, owner, repo string, opts provider.MarkNotificationsOptions) error {
	markOpts := gitea.MarkNotificationOptions{}
	if opts.LastReadAt != "" {
		markOpts.LastReadAt, _ = parseTime(opts.LastReadAt)
	}
	_, _, err := p.client.ReadRepoNotifications(owner, repo, markOpts)
	return provider.Wrap(provider.PlatformGitea, "MarkRepoNotificationsRead", err)
}

func convertNotification(t *gitea.NotificationThread) *provider.Notification {
	n := &provider.Notification{
		ID:     strconv.FormatInt(t.ID, 10),
		Unread: t.Unread,
	}
	if t.Subject != nil {
		n.Subject = provider.NotificationSubject{
			Title: t.Subject.Title,
			Type:  string(t.Subject.Type),
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

// parseTime is a helper that parses an RFC3339 timestamp. It returns the
// zero value on parse failure so callers can degrade gracefully.
func parseTime(s string) (t time.Time, err error) {
	return time.Parse(time.RFC3339, s)
}

var _ provider.NotificationManager = (*Provider)(nil)

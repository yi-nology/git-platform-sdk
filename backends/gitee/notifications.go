package gitee

import (
	"context"
	"strconv"
	"time"

	gitee "gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListNotifications implements provider.NotificationManager.
func (p *Provider) ListNotifications(ctx context.Context, opts provider.ListNotificationsOptions) ([]*provider.Notification, error) {
	giteeOpts := &gitee.GetV5NotificationsThreadsOpts{
		AccessToken: p.accessToken(),
	}
	if opts.Since != "" {
		giteeOpts.Since = optional.NewString(opts.Since)
	}
	if opts.Page > 0 {
		giteeOpts.Page = optional.NewInt32(int32(opts.Page))
	}
	if opts.PerPage > 0 {
		giteeOpts.PerPage = optional.NewInt32(int32(opts.PerPage))
	}
	lists, resp, err := p.client.ActivityApi.GetV5NotificationsThreads(ctx, giteeOpts)
	if err != nil {
		return nil, p.sdkErr("ListNotifications", resp, err)
	}
	return flattenNotificationLists(lists), nil
}

// ListRepoNotifications implements provider.NotificationManager.
func (p *Provider) ListRepoNotifications(ctx context.Context, owner, repo string, opts provider.ListNotificationsOptions) ([]*provider.Notification, error) {
	giteeOpts := &gitee.GetV5ReposOwnerRepoNotificationsOpts{
		AccessToken: p.accessToken(),
	}
	if opts.Since != "" {
		giteeOpts.Since = optional.NewString(opts.Since)
	}
	if opts.Page > 0 {
		giteeOpts.Page = optional.NewInt32(int32(opts.Page))
	}
	if opts.PerPage > 0 {
		giteeOpts.PerPage = optional.NewInt32(int32(opts.PerPage))
	}
	lists, resp, err := p.client.ActivityApi.GetV5ReposOwnerRepoNotifications(ctx, esc(owner), esc(repo), giteeOpts)
	if err != nil {
		return nil, p.sdkErr("ListRepoNotifications", resp, err)
	}
	return flattenNotificationLists(lists), nil
}

// MarkNotificationRead implements provider.NotificationManager.
func (p *Provider) MarkNotificationRead(ctx context.Context, threadID string) error {
	resp, err := p.client.ActivityApi.PatchV5NotificationsThreadsId(ctx, esc(threadID), &gitee.PatchV5NotificationsThreadsIdOpts{
		AccessToken: p.accessToken(),
	})
	return p.sdkErr("MarkNotificationRead", resp, err)
}

// MarkNotificationsRead implements provider.NotificationManager.
func (p *Provider) MarkNotificationsRead(ctx context.Context, opts provider.MarkNotificationsOptions) error {
	resp, err := p.client.ActivityApi.PutV5NotificationsThreads(ctx, &gitee.PutV5NotificationsThreadsOpts{
		AccessToken: p.accessToken(),
	})
	return p.sdkErr("MarkNotificationsRead", resp, err)
}

// MarkRepoNotificationsRead implements provider.NotificationManager.
func (p *Provider) MarkRepoNotificationsRead(ctx context.Context, owner, repo string, opts provider.MarkNotificationsOptions) error {
	resp, err := p.client.ActivityApi.PutV5ReposOwnerRepoNotifications(ctx, esc(owner), esc(repo), &gitee.PutV5ReposOwnerRepoNotificationsOpts{
		AccessToken: p.accessToken(),
	})
	return p.sdkErr("MarkRepoNotificationsRead", resp, err)
}

// flattenNotificationLists flattens the Gitee nested []UserNotificationList
// into a flat []*provider.Notification.
func flattenNotificationLists(lists []gitee.UserNotificationList) []*provider.Notification {
	var result []*provider.Notification
	for _, nl := range lists {
		for _, n := range nl.List {
			result = append(result, convertNotification(n))
		}
	}
	return result
}

func convertNotification(n gitee.UserNotification) *provider.Notification {
	out := &provider.Notification{
		ID:     strconv.FormatInt(int64(n.Id), 10),
		Unread: n.Unread == "true",
	}
	if n.Subject != nil {
		out.Subject = provider.NotificationSubject{
			Title: n.Subject.Title,
			Type:  n.Subject.Type_,
			URL:   n.Subject.Url,
		}
	}
	if n.Repository != nil {
		out.Repo = &provider.EventRepo{
			ID:       int64(n.Repository.Id),
			FullName: n.Repository.FullName,
		}
		owner, name := provider.SplitFullName(n.Repository.FullName)
		out.Repo.Owner = owner
		out.Repo.Name = name
	}
	if t, err := time.Parse(time.RFC3339, n.UpdatedAt); err == nil {
		out.UpdatedAt = t
	}
	return out
}

var _ provider.NotificationManager = (*Provider)(nil)

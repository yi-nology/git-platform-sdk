package provider

import "context"

// NotificationManager provides access to a user's notification inbox and
// per-repository notification streams. It is an optional capability interface:
// consumers should gate on Provider.Capabilities().Notifications (or
// type-assert) before use.
//
// Platform support: GitHub, GitCode, Gitea, Forgejo, Gitee. GitLab exposes
// only notification *settings* (no inbox) and TencentCode has no notification
// API at all — both report Notifications=false in CapabilitySet.
type NotificationManager interface {
	// ListNotifications returns the authenticated user's notifications,
	// exhausting the platform's pagination.
	ListNotifications(ctx context.Context, opts ListNotificationsOptions) ([]*Notification, error)
	// ListRepoNotifications returns notifications for a specific repository.
	ListRepoNotifications(ctx context.Context, owner, repo string, opts ListNotificationsOptions) ([]*Notification, error)
	// MarkNotificationRead marks a single notification thread as read.
	MarkNotificationRead(ctx context.Context, threadID string) error
	// MarkNotificationsRead marks all (or filtered) notifications as read.
	MarkNotificationsRead(ctx context.Context, opts MarkNotificationsOptions) error
	// MarkRepoNotificationsRead marks all notifications for a repository as read.
	MarkRepoNotificationsRead(ctx context.Context, owner, repo string, opts MarkNotificationsOptions) error
}

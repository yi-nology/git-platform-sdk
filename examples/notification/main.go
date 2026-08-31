// Example: list notifications and mark them as read.
//
// Set PLATFORM_TOKEN to run. Works with GitHub, GitCode, GitLab, Gitea,
// Forgejo, and Gitee.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yi-nology/git-platform-sdk/provider"
)

func main() {
	token := os.Getenv("PLATFORM_TOKEN")
	if token == "" {
		log.Fatal("set PLATFORM_TOKEN to run this example")
	}

	p, err := provider.NewProvider(provider.Config{
		Platform: provider.PlatformGitHub,
		Token:    token,
	})
	if err != nil {
		log.Fatalf("new provider: %v", err)
	}

	// NotificationManager is an optional capability.
	nm, ok := p.(provider.NotificationManager)
	if !ok {
		fmt.Println("notifications not supported on this platform")
		return
	}

	ctx := context.Background()

	notifications, err := nm.ListNotifications(ctx, provider.ListNotificationsOptions{})
	if err != nil {
		log.Fatalf("list notifications: %v", err)
	}

	unread := 0
	for _, n := range notifications {
		if n.Unread {
			unread++
		}
		fmt.Printf("[%s] %s: %s\n", n.Repo.FullName, n.Subject.Type, n.Subject.Title)
	}
	fmt.Printf("\n%d total, %d unread\n", len(notifications), unread)

	// Mark the first unread notification as read.
	if len(notifications) > 0 && notifications[0].Unread {
		if err := nm.MarkNotificationRead(ctx, notifications[0].ID); err != nil {
			log.Printf("mark read: %v", err)
		} else {
			fmt.Printf("marked notification %s as read\n", notifications[0].ID)
		}
	}
}

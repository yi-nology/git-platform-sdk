package provider

import (
	"context"
	"net/http"
)

// WebhookManager handles webhook CRUD and event parsing.
type WebhookManager interface {
	CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error)
	DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error
	ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error)
	ParseWebhookEvent(r *http.Request, secret string) (*NormalizedEvent, error)
	ValidateWebhookSignature(r *http.Request, secret string) error
}

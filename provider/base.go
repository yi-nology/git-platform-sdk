package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type authStyle int

const (
	authHeaderPrivateToken authStyle = iota
	authHeaderBearer
	authHeaderToken
)

type baseProvider struct {
	baseURL     string
	token       string
	client      *http.Client
	auth        authStyle
	errPrefix   string
	logger      Logger
	retryConfig *RetryConfig
	hooks       *Hooks
}

func newBaseProvider(baseURL, token string, skipTLS bool, auth authStyle, errPrefix string, opts ...baseOption) *baseProvider {
	transport := &http.Transport{}
	if skipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	bp := &baseProvider{
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     token,
		client:    &http.Client{Timeout: 30 * time.Second, Transport: transport},
		auth:      auth,
		errPrefix: errPrefix,
		logger:    NewNoopLogger(),
	}
	for _, opt := range opts {
		opt(bp)
	}
	return bp
}

// baseOption configures a baseProvider.
type baseOption func(*baseProvider)

// withLogger sets the logger for the base provider.
func withLogger(l Logger) baseOption {
	return func(bp *baseProvider) {
		if l != nil {
			bp.logger = l
		}
	}
}

// withRetry sets the retry configuration.
func withRetry(rc RetryConfig) baseOption {
	return func(bp *baseProvider) {
		bp.retryConfig = &rc
	}
}

// withHooks sets the request/response hooks.
func withHooks(h *Hooks) baseOption {
	return func(bp *baseProvider) {
		bp.hooks = h
	}
}

// configBaseOptions extracts baseOption slice from a Config.
func configBaseOptions(cfg Config) []baseOption {
	var opts []baseOption
	if cfg.Logger != nil {
		opts = append(opts, withLogger(cfg.Logger))
	}
	if cfg.RetryConfig != nil {
		opts = append(opts, withRetry(*cfg.RetryConfig))
	}
	if cfg.Hooks != nil {
		opts = append(opts, withHooks(cfg.Hooks))
	}
	return opts
}

func (b *baseProvider) setAuthHeader(req *http.Request) {
	switch b.auth {
	case authHeaderBearer:
		req.Header.Set("Authorization", "Bearer "+b.token)
	case authHeaderToken:
		req.Header.Set("Authorization", "token "+b.token)
	default:
		req.Header.Set("PRIVATE-TOKEN", b.token)
	}
}

func (b *baseProvider) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	_, err := b.doRequestWithHeaders(ctx, method, path, body, result)
	return err
}

func (b *baseProvider) doRequestWithHeaders(ctx context.Context, method, path string, body interface{}, result interface{}) (http.Header, error) {
	var reqBody io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	b.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	// Execute request hooks
	ctx = b.hooks.executeRequestHooks(ctx, req)

	start := time.Now()
	var resp *http.Response
	var respBody []byte

	if b.retryConfig != nil && b.retryConfig.MaxRetries > 0 {
		resp, respBody, err = b.retryableDo(ctx, req)
	} else {
		resp, err = b.client.Do(req)
		if err == nil {
			respBody, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
		}
	}

	duration := time.Since(start)

	// Execute response hooks
	b.hooks.executeResponseHooks(ctx, req, resp, duration, err)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, NewProviderError(Platform(b.errPrefix), fmt.Sprintf("%s %s", method, path), resp.StatusCode, string(respBody))
	}
	if result != nil && resp.StatusCode != http.StatusNoContent {
		return resp.Header, json.Unmarshal(respBody, result)
	}
	return resp.Header, nil
}

func (b *baseProvider) doRawRequest(ctx context.Context, method, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, b.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	b.setAuthHeader(req)

	// Execute request hooks
	ctx = b.hooks.executeRequestHooks(ctx, req)

	start := time.Now()
	var resp *http.Response
	var body []byte

	if b.retryConfig != nil && b.retryConfig.MaxRetries > 0 {
		resp, body, err = b.retryableDo(ctx, req)
	} else {
		resp, err = b.client.Do(req)
		if err == nil {
			body, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
		}
	}

	duration := time.Since(start)

	// Execute response hooks
	b.hooks.executeResponseHooks(ctx, req, resp, duration, err)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, NewProviderError(Platform(b.errPrefix), fmt.Sprintf("%s %s", method, path), resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	return body, nil
}

// readAndRestoreBody reads the response body and replaces it with a new NopCloser
// so it can be read again (needed for retry logic).
func readAndRestoreBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

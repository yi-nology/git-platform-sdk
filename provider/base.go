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
	baseURL   string
	token     string
	client    *http.Client
	auth      authStyle
	errPrefix string
}

func newBaseProvider(baseURL, token string, skipTLS bool, auth authStyle, errPrefix string) *baseProvider {
	transport := &http.Transport{}
	if skipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &baseProvider{
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     token,
		client:    &http.Client{Timeout: 30 * time.Second, Transport: transport},
		auth:      auth,
		errPrefix: errPrefix,
	}
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
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s API %s %s returned %d: %s", b.errPrefix, method, path, resp.StatusCode, string(respBody))
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
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s API %s %s returned %d: %s", b.errPrefix, method, path, resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	return body, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

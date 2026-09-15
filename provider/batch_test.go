package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestBatchResultOrderAndAttribution(t *testing.T) {
	// odd-length paths fail, even-length succeed — order must follow input
	paths := []string{"a", "bb", "ccc"}
	got := GetFileContents(context.Background(), batchFakeProvider{}, "o", "r", "", paths,
		BatchOptions{Concurrency: 3})
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for i, r := range got {
		if r.Key != paths[i] {
			t.Fatalf("result %d key = %q, want %q", i, r.Key, paths[i])
		}
	}
	if got[0].Err == nil || got[2].Err == nil {
		t.Fatalf("odd items must fail: %+v", got)
	}
	if got[1].Err != nil || got[1].Value != "content:bb" {
		t.Fatalf("bb must succeed: %+v", got[1])
	}
}

func TestBatchConcurrencyBound(t *testing.T) {
	var mu sync.Mutex
	inFlight, peak := 0, 0
	p := countingFakeProvider{hook: func() {
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		mu.Lock()
		inFlight--
		mu.Unlock()
	}}

	paths := make([]string, 8)
	for i := range paths {
		paths[i] = fmt.Sprintf("f%d", i)
	}
	_ = GetFileContents(context.Background(), p, "o", "r", "", paths, BatchOptions{Concurrency: 2})
	mu.Lock()
	defer mu.Unlock()
	if peak > 2 {
		t.Fatalf("peak concurrency = %d, want <= 2", peak)
	}
}

func TestBatchContextCancellationFailsRemaining(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := GetCRs(ctx, batchFakeProvider{}, "o", "r", []string{"1", "2", "3"})
	for i, r := range got {
		if !errors.Is(r.Err, context.Canceled) {
			t.Fatalf("result %d err = %v, want context.Canceled", i, r.Err)
		}
	}
}

// notImplementedProvider satisfies Provider via nil-embedding but has no
// CommitStatusManager (an optional capability, not part of Provider).
type notImplementedProvider struct{ Provider }

func (notImplementedProvider) Platform() Platform { return PlatformGitHub }

func TestBatchNotImplementedCapability(t *testing.T) {
	p := notImplementedProvider{}
	got2 := SetCommitStatuses(context.Background(), p, "o", "r", "sha", []CommitStatusOptions{{Context: "ci"}})
	if len(got2) != 1 || !errors.Is(got2[0].Err, ErrNotImplemented) {
		t.Fatalf("err = %v, want ErrNotImplemented", got2[0].Err)
	}
}

func TestBatchSetCommitStatusesKeys(t *testing.T) {
	p := statusFakeProvider{}
	got := SetCommitStatuses(context.Background(), p, "o", "r", "sha", []CommitStatusOptions{
		{Context: "ci/lint"}, {Context: "ci/test"},
	})
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Key != "ci/lint" || got[1].Key != "ci/test" {
		t.Fatalf("keys = %q %q, want contexts", got[0].Key, got[1].Key)
	}
	if got[0].Err != nil || got[1].Err != nil {
		t.Fatalf("unexpected errs: %v %v", got[0].Err, got[1].Err)
	}
}

// --- fakes ---

type batchFakeProvider struct{ Provider }

func (batchFakeProvider) Platform() Platform { return PlatformGitHub }
func (batchFakeProvider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	if len(path)%2 == 1 {
		return "", New(PlatformGitHub, "GetFileContent", http.StatusNotFound, "no such file")
	}
	return "content:" + path, nil
}
func (batchFakeProvider) GetCR(ctx context.Context, owner, repo, number string) (*ChangeRequest, error) {
	return &ChangeRequest{Number: number}, nil
}

type countingFakeProvider struct {
	Provider
	hook func()
}

func (p countingFakeProvider) Platform() Platform { return PlatformGitHub }
func (p countingFakeProvider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	p.hook()
	return "", nil
}

type statusFakeProvider struct{ Provider }

func (statusFakeProvider) Platform() Platform { return PlatformGitHub }
func (statusFakeProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	return nil
}
func (statusFakeProvider) ListCommitStatuses(ctx context.Context, owner, repo, sha string) ([]CommitStatus, error) {
	return nil, nil
}

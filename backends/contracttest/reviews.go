package contracttest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ReviewsHarnessConfig carries the fixtures a backend's main Harness needs to
// auto-mount the review-management suite via Harness.Reviews.
type ReviewsHarnessConfig struct {
	// ListResponse is the JSON array for review-list GETs. First item: id 1,
	// user login "dev", state "APPROVED" (the suite asserts normalization to
	// provider.ReviewStateApproved).
	ListResponse string
	// GetResponse is the JSON object for single-review GETs (id 1, user
	// "dev", state "APPROVED").
	GetResponse string
	// MutateResponse is the JSON object for POST/PUT/PATCH (same shape as
	// GetResponse).
	MutateResponse string
	// IgnoresRequestReviewers opts the backend out of the RequestReviewers
	// wire assertion (see ReviewsHarness.IgnoresRequestReviewers).
	IgnoresRequestReviewers bool
}

// ReviewsHarness is the full harness RunReviewsSuite consumes; auto-mounting
// builds it from the enclosing Harness plus ReviewsHarnessConfig.
type ReviewsHarness struct {
	Name        string
	Platform    provider.Platform
	NewProvider func(t *testing.T, cfg provider.Config) provider.Provider
	ReviewsHarnessConfig
	// IgnoresRequestReviewers declares that the backend's RequestReviewers is
	// a registered ignore: the platform's reviewer API needs inputs the SDK
	// surface cannot supply (GitLab's reviewer_ids want user IDs, but the SDK
	// addresses reviewers by username and exposes no Users API to resolve
	// them), so the method documents the ignore and performs no request. The
	// wire subtest then only asserts a silent, error-free no-op — anything
	// reaching the network would mean the registration drifted.
	IgnoresRequestReviewers bool
}

// testReviewsSuite auto-mounts RunReviewsSuite from a main Harness with the
// same bidirectional drift checks as the labels and issues suites.
func testReviewsSuite(t *testing.T, h Harness) {
	srv := httptest.NewServer(stubHandler(h))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	declared := p.Capabilities().Reviews
	switch {
	case h.Reviews == nil && !declared:
		t.Skipf("%s declares no Reviews capability", h.Name)
	case h.Reviews == nil:
		t.Errorf("%s declares Capabilities().Reviews but its Harness provides no Reviews config — the reviews suite is not wired", h.Name)
	case !declared:
		t.Errorf("%s Harness provides a Reviews config but the platform does not declare Capabilities().Reviews", h.Name)
	default:
		RunReviewsSuite(t, ReviewsHarness{
			Name:                    h.Name,
			Platform:                h.Platform,
			NewProvider:             h.NewProvider,
			ReviewsHarnessConfig:    *h.Reviews,
			IgnoresRequestReviewers: h.Reviews.IgnoresRequestReviewers,
		})
	}
}

// RunReviewsSuite executes the review-management contract suite. The mock
// routes by method and path shape so platform-specific paths don't matter:
// GET ending /reviews/{digits} → GetResponse; other GET → ListResponse;
// POST → 201 + MutateResponse; PUT/PATCH → MutateResponse (GitHub's dismiss
// is a PUT to a .../dismissals sub-resource); DELETE → 204. Requests are
// recorded for wire assertions.
func RunReviewsSuite(t *testing.T, h ReviewsHarness) {
	newRM := func(t *testing.T) (provider.ReviewManager, *[]recordedRequest) {
		srv, requests := reviewStubServer(h)
		t.Cleanup(srv.Close)
		p := h.NewProvider(t, provider.Config{Platform: h.Platform, BaseURL: srv.URL, Token: "test"})
		rm, ok := p.(provider.ReviewManager)
		if !ok {
			t.Fatalf("%s does not implement provider.ReviewManager", h.Name)
		}
		return rm, requests
	}

	t.Run("List_ParsesAndNormalizes", func(t *testing.T) {
		rm, _ := newRM(t)
		assertReviewListNormalized(t, rm)
	})
	t.Run("Get_ReturnsReview", func(t *testing.T) {
		rm, _ := newRM(t)
		assertReviewGet(t, rm)
	})
	t.Run("Create_PostsBody", func(t *testing.T) {
		rm, requests := newRM(t)
		assertReviewCreateWire(t, rm, requests)
	})
	t.Run("RequestReviewers_PostsReviewers", func(t *testing.T) {
		rm, requests := newRM(t)
		if h.IgnoresRequestReviewers {
			assertRequestReviewersIgnored(t, rm, requests)
			return
		}
		assertRequestReviewersWire(t, rm, requests)
	})
	t.Run("Dismiss_NotGet", func(t *testing.T) {
		rm, requests := newRM(t)
		assertReviewDismissWire(t, rm, requests)
	})
}

// assertReviewListNormalized checks that ListReviews returns parsed,
// normalized reviews: id 1, user "dev", and the UPPERCASE wire state
// "APPROVED" normalized to provider.ReviewStateApproved.
func assertReviewListNormalized(t *testing.T, rm provider.ReviewManager) {
	t.Helper()
	reviews, err := rm.ListReviews(context.Background(), "owner", "repo", "1")
	if err != nil {
		t.Fatalf("ListReviews: %v", err)
	}
	if len(reviews) == 0 {
		t.Fatal("expected at least one review")
	}
	if reviews[0].ID != 1 {
		t.Errorf("expected first review id 1, got %d", reviews[0].ID)
	}
	if reviews[0].User != "dev" {
		t.Errorf("expected review user %q, got %q", "dev", reviews[0].User)
	}
	if reviews[0].State != provider.ReviewStateApproved {
		t.Errorf("expected normalized state %q, got %q", provider.ReviewStateApproved, reviews[0].State)
	}
}

// assertReviewGet checks that GetReview returns the fixture review.
func assertReviewGet(t *testing.T, rm provider.ReviewManager) {
	t.Helper()
	review, err := rm.GetReview(context.Background(), "owner", "repo", "1", 1)
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	if review == nil || review.ID != 1 {
		t.Fatalf("expected review id 1, got %+v", review)
	}
}

// assertReviewCreateWire checks that CreateReview succeeds and posts a
// review-ish payload: the summary body under the "body" key (the key GitHub
// and GitHub-shaped platforms send).
func assertReviewCreateWire(t *testing.T, rm provider.ReviewManager, requests *[]recordedRequest) {
	t.Helper()
	result, err := rm.CreateReview(context.Background(), "owner", "repo", "1", provider.CreateReviewOptions{Body: "looks good"})
	if err != nil || result == nil {
		t.Fatalf("CreateReview: result=%v err=%v", result, err)
	}
	assertBodyHas(t, requests, http.MethodPost, "body", "looks good")
}

// assertRequestReviewersWire checks that RequestReviewers posts the reviewer
// logins under the "reviewers" key. The value is a JSON array, so the string
// assertion in assertBodyHas does not apply.
func assertRequestReviewersWire(t *testing.T, rm provider.ReviewManager, requests *[]recordedRequest) {
	t.Helper()
	if err := rm.RequestReviewers(context.Background(), "owner", "repo", "1", []string{"dev"}); err != nil {
		t.Fatalf("RequestReviewers: %v", err)
	}
	if !sawMethod(*requests, http.MethodPost) {
		t.Fatalf("expected a POST request, recorded %s", methodsOf(*requests))
	}
	for i := range *requests {
		r := &(*requests)[i]
		if r.Method != http.MethodPost {
			continue
		}
		if v, ok := r.Body["reviewers"]; ok {
			if items, ok := v.([]any); ok && len(items) > 0 {
				for _, item := range items {
					if s, ok := item.(string); ok && strings.Contains(s, "dev") {
						return
					}
				}
			}
		}
	}
	t.Errorf("expected a POST whose body carries reviewers=[\"dev\"], recorded: %v", *requests)
}

// assertRequestReviewersIgnored checks a registered-ignore RequestReviewers:
// the call must succeed and stay completely off the wire (GitLab; see
// ReviewsHarness.IgnoresRequestReviewers). Any recorded request means the
// ignore registration has drifted from the implementation.
func assertRequestReviewersIgnored(t *testing.T, rm provider.ReviewManager, requests *[]recordedRequest) {
	t.Helper()
	if err := rm.RequestReviewers(context.Background(), "owner", "repo", "1", []string{"dev"}); err != nil {
		t.Fatalf("RequestReviewers (registered ignore): %v", err)
	}
	if len(*requests) != 0 {
		t.Errorf("expected the registered ignore to make no HTTP requests, recorded %s", methodsOf(*requests))
	}
}

// assertReviewDismissWire checks that DismissReview reached the server with a
// state-changing verb. Platforms differ: GitHub PUTs a dismissal sub-resource,
// others DELETE the review. A GET-only recording means the dismissal never
// mutated anything.
func assertReviewDismissWire(t *testing.T, rm provider.ReviewManager, requests *[]recordedRequest) {
	t.Helper()
	if err := rm.DismissReview(context.Background(), "owner", "repo", "1", 1, "stale review"); err != nil {
		t.Fatalf("DismissReview: %v", err)
	}
	if len(*requests) == 0 {
		t.Fatal("expected the dismissal to reach the mock server")
	}
	for i := range *requests {
		if (*requests)[i].Method != http.MethodGet {
			return
		}
	}
	t.Errorf("expected a non-GET dismissal request, recorded %s", methodsOf(*requests))
}

// reviewPathID matches a path that addresses a single review, e.g.
// "/repos/owner/repo/pulls/1/reviews/42".
var reviewPathID = regexp.MustCompile(`/reviews/\d+$`)

// reviewStubServer returns the method/shape-routed recording mock.
func reviewStubServer(h ReviewsHarness) (*httptest.Server, *[]recordedRequest) {
	var mu sync.Mutex
	var requests []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := decodeRecordedBody(r)
		mu.Lock()
		requests = append(requests, recordedRequest{Method: r.Method, Path: r.URL.RequestURI(), Body: body})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(h.MutateResponse))
		case r.Method == http.MethodPatch || r.Method == http.MethodPut:
			_, _ = w.Write([]byte(h.MutateResponse))
		case r.Method == http.MethodGet && reviewPathID.MatchString(r.URL.Path):
			_, _ = w.Write([]byte(h.GetResponse))
		default:
			_, _ = w.Write([]byte(h.ListResponse))
		}
	}))
	return srv, &requests
}

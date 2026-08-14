package contracttest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// testCapabilities asserts that a provider's declared CapabilitySet matches
// the optional interfaces its concrete type actually implements. A declared
// capability must type-assert successfully, and an undeclared one must not.
// This keeps backend declarations from drifting from their method sets as
// new capability interfaces land. When a new optional interface is added to
// the SDK (e.g. MilestoneManager, ReviewManager), extend the checks here.
func testCapabilities(t *testing.T, h Harness) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(h.EmptyListResponse))
	}))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	caps := p.Capabilities()

	_, issuesImpl := p.(provider.IssueManager)
	_, searchImpl := p.(provider.SearchManager)
	_, labelsImpl := p.(provider.LabelManager)

	if caps.Issues != issuesImpl {
		t.Errorf("Capabilities().Issues = %v, but IssueManager type assertion = %v; declaration and implementation have drifted", caps.Issues, issuesImpl)
	}
	if caps.Search != searchImpl {
		t.Errorf("Capabilities().Search = %v, but SearchManager type assertion = %v; declaration and implementation have drifted", caps.Search, searchImpl)
	}
	if caps.Labels != labelsImpl {
		t.Errorf("Capabilities().Labels = %v, but LabelManager type assertion = %v; declaration and implementation have drifted", caps.Labels, labelsImpl)
	}
}

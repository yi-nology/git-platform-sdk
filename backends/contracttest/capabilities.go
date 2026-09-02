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
	_, reviewsImpl := p.(provider.ReviewManager)
	_, milestonesImpl := p.(provider.MilestoneManager)
	_, commitStatusesImpl := p.(provider.CommitStatusManager)
	_, notificationsImpl := p.(provider.NotificationManager)
	_, reactionsImpl := p.(provider.ReactionManager)
	_, branchProtectionsImpl := p.(provider.BranchProtectionManager)
	_, collaboratorsImpl := p.(provider.CollaboratorManager)
	_, deployKeysImpl := p.(provider.DeploymentKeyManager)
	_, repoStatsImpl := p.(provider.RepoStatsManager)
	_, usersImpl := p.(provider.UserManager)

	// Declared capabilities must always type-assert, and implemented ones
	// must be declared — both directions, uniformly across capabilities.
	if caps.Issues != issuesImpl {
		t.Errorf("Capabilities().Issues = %v, but IssueManager type assertion = %v; declaration and implementation have drifted", caps.Issues, issuesImpl)
	}
	if caps.Search != searchImpl {
		t.Errorf("Capabilities().Search = %v, but SearchManager type assertion = %v; declaration and implementation have drifted", caps.Search, searchImpl)
	}
	if caps.Labels != labelsImpl {
		t.Errorf("Capabilities().Labels = %v, but LabelManager type assertion = %v; declaration and implementation have drifted", caps.Labels, labelsImpl)
	}
	if caps.Reviews != reviewsImpl {
		t.Errorf("Capabilities().Reviews = %v, but ReviewManager type assertion = %v; declaration and implementation have drifted", caps.Reviews, reviewsImpl)
	}
	if caps.Milestones != milestonesImpl {
		t.Errorf("Capabilities().Milestones = %v, but MilestoneManager type assertion = %v; declaration and implementation have drifted", caps.Milestones, milestonesImpl)
	}
	if caps.CommitStatuses != commitStatusesImpl {
		t.Errorf("Capabilities().CommitStatuses = %v, but CommitStatusManager type assertion = %v; declaration and implementation have drifted", caps.CommitStatuses, commitStatusesImpl)
	}
	if caps.Notifications != notificationsImpl {
		t.Errorf("Capabilities().Notifications = %v, but NotificationManager type assertion = %v; declaration and implementation have drifted", caps.Notifications, notificationsImpl)
	}
	if caps.Reactions != reactionsImpl {
		t.Errorf("Capabilities().Reactions = %v, but ReactionManager type assertion = %v; declaration and implementation have drifted", caps.Reactions, reactionsImpl)
	}
	if caps.BranchProtections != branchProtectionsImpl {
		t.Errorf("Capabilities().BranchProtections = %v, but BranchProtectionManager type assertion = %v; declaration and implementation have drifted", caps.BranchProtections, branchProtectionsImpl)
	}
	if caps.Collaborators != collaboratorsImpl {
		t.Errorf("Capabilities().Collaborators = %v, but CollaboratorManager type assertion = %v; declaration and implementation have drifted", caps.Collaborators, collaboratorsImpl)
	}
	if caps.DeployKeys != deployKeysImpl {
		t.Errorf("Capabilities().DeployKeys = %v, but DeploymentKeyManager type assertion = %v; declaration and implementation have drifted", caps.DeployKeys, deployKeysImpl)
	}
	if caps.RepoStats != repoStatsImpl {
		t.Errorf("Capabilities().RepoStats = %v, but RepoStatsManager type assertion = %v; declaration and implementation have drifted", caps.RepoStats, repoStatsImpl)
	}
	if caps.Users != usersImpl {
		t.Errorf("Capabilities().Users = %v, but UserManager type assertion = %v; declaration and implementation have drifted", caps.Users, usersImpl)
	}
}

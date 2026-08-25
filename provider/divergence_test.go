package provider

import "testing"

func TestFindByMethod(t *testing.T) {
	divs := []Divergence{
		{Capability: "ReviewManager", Method: "RequestReviewers", Kind: DivergenceIgnore},
		{Capability: "ReviewManager", Method: "DismissReview", Kind: DivergenceStub},
	}
	if got := FindByMethod(divs, "RequestReviewers"); len(got) != 1 || got[0].Kind != DivergenceIgnore {
		t.Fatalf("FindByMethod = %+v, want the single ignore entry", got)
	}
	if got := FindByMethod(divs, "Missing"); len(got) != 0 {
		t.Fatalf("FindByMethod(Missing) = %+v, want none", got)
	}
}

func TestIgnores(t *testing.T) {
	divs := []Divergence{
		{Capability: "IssueManager", Method: "CreateIssue", Field: "opts.Assignees", Kind: DivergenceIgnore},
	}
	if !Ignores(divs, "CreateIssue", "opts.Assignees") {
		t.Fatal("Ignores(CreateIssue, opts.Assignees) = false, want true")
	}
	if Ignores(divs, "CreateIssue", "opts.Title") {
		t.Fatal("Ignores(CreateIssue, opts.Title) = true, want false")
	}
	if Ignores(divs, "UpdateIssue", "opts.Assignees") {
		t.Fatal("Ignores on an unregistered method = true, want false")
	}
}

func TestStubs(t *testing.T) {
	divs := []Divergence{{Capability: "CommitManager", Method: "CreateCommitStatus", Kind: DivergenceStub}}
	if !Stubs(divs, "CreateCommitStatus") {
		t.Fatal("Stubs(CreateCommitStatus) = false, want true")
	}
	if Stubs(divs, "GetCommit") {
		t.Fatal("Stubs(GetCommit) = true, want false")
	}
}

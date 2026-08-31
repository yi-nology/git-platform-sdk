package provider

import (
	"regexp"
	"strings"
)

// mentionRe matches @username patterns that are NOT preceded by a word
// character (to avoid matching email addresses like user@domain.com).
// Valid usernames contain letters, digits, underscores, hyphens, and dots
// (the last to cover platforms like GitLab that use @first.last).
var mentionRe = regexp.MustCompile(`(?:^|[\s\p{P}])@([\w.-]+)`)

// ExtractMentions returns the deduplicated list of @usernames found in body.
// Email addresses (foo@bar.com) are excluded by the leading non-word-char
// guard. The returned order follows first occurrence.
func ExtractMentions(body string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, m := range mentionRe.FindAllStringSubmatch(body, -1) {
		name := m[1]
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}
	return result
}

// SplitFullName splits "owner/repo" into (owner, repo).
// If the input doesn't contain "/", owner is empty.
func SplitFullName(fullName string) (owner, name string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", fullName
}

// ExtractOwnerFromFullName returns just the owner portion of "owner/repo".
func ExtractOwnerFromFullName(fullName string) string {
	owner, _ := SplitFullName(fullName)
	return owner
}

// BuildEventRepo creates an EventRepo from a full name string.
func BuildEventRepo(fullName string) *EventRepo {
	owner, name := SplitFullName(fullName)
	return &EventRepo{
		FullName: fullName,
		Owner:    owner,
		Name:     name,
	}
}

// ResolveMRSHAs derives the (head, base, start) SHAs for a GitLab-style merge
// request from the raw webhook fields. This encodes the shared priority used by
// the GitLab, TencentCode (and future GitCode) backends:
//
//   - head: diff_refs.head_sha when present, otherwise last_commit.id
//   - base: merge_commit_sha when present, otherwise diff_refs.base_sha
//   - start: diff_refs.start_sha (may be empty)
//
// Keeping this in one place guarantees identical fallback semantics across the
// GitLab-family backends and avoids behavioural drift.
func ResolveMRSHAs(diffRefsHead, diffRefsBase, diffRefsStart, mergeCommitSHA, lastCommitID string) (head, base, start string) {
	head = lastCommitID
	if diffRefsHead != "" {
		head = diffRefsHead
	}
	base = mergeCommitSHA
	if base == "" {
		base = diffRefsBase
	}
	start = diffRefsStart
	return head, base, start
}

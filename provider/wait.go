package provider

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Default polling parameters for WaitForCommitStatus.
const (
	DefaultWaitInterval = 5 * time.Second
	DefaultWaitTimeout  = 10 * time.Minute
	minWaitInterval     = 250 * time.Millisecond
)

// WaitOptions tunes the polling loop in WaitForCommitStatus.
type WaitOptions struct {
	// Interval is the poll cadence; defaults to DefaultWaitInterval and
	// is clamped up to minWaitInterval.
	Interval time.Duration
	// Timeout bounds the whole wait; defaults to DefaultWaitTimeout. The
	// returned error wraps ErrDeadlineExceeded-style context deadlines —
	// pass Timeout <= 0 with your own context deadline to disable it.
	Timeout time.Duration
	// Contexts, when non-empty, restricts the combined state to the
	// named status contexts (e.g. []string{"ci/lint", "ci/test"}).
	// Empty means every context reported on the commit.
	Contexts []string
	// InitialDelay defers the first poll; useful right after
	// CreateCommitStatus, whose result may not be queryable for a
	// moment. Defaults to no delay.
	InitialDelay time.Duration
}

// ErrWaitTimedOut is returned by WaitForCommitStatus when the commit
// stays non-terminal past WaitOptions.Timeout.
var ErrWaitTimedOut = errors.New("commit status did not reach a terminal state before the timeout")

// WaitForCommitStatus polls ListCommitStatuses until the commit's
// combined state is terminal (success/failure/error/canceled) and returns
// it. It is the CI-gate primitive for automation and agent flows: create
// your status (or push a commit), wait, then decide.
//
// The provider must implement CommitStatusManager; otherwise the error
// wraps ErrNotImplemented. Context cancellation is honored between and
// during polls. A combined state of pending/running past the timeout
// fails with ErrWaitTimedOut (so a red build is never confused with "gave
// up waiting"): distinguish them via errors.Is.
//
// States are combined with CombineCommitStatusStates semantics — any
// error beats failure beats canceled; any pending/running keeps waiting;
// an all-success set succeeds. With WaitOptions.Contexts set, only the
// named contexts are folded; contexts that have not reported yet are
// simply absent, so the wait continues until each named context lands a
// terminal state.
func WaitForCommitStatus(ctx context.Context, p Provider, owner, repo, sha string, opts WaitOptions) (CommitStatusState, error) {
	csm, ok := p.(CommitStatusManager)
	if !ok {
		// no p.Platform() here: callers may pass a partially-implemented
		// (embedded-interface) provider whose Platform() would panic.
		return "", fmt.Errorf("%w: provider does not implement CommitStatusManager", ErrNotImplemented)
	}

	interval, budget, want := initWait(opts)

	if opts.InitialDelay > 0 {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-budget:
			return "", ErrWaitTimedOut
		case <-time.After(opts.InitialDelay):
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		statuses, err := csm.ListCommitStatuses(ctx, owner, repo, sha)
		if err == nil {
			if state, done := foldTerminalStates(statuses, want); done {
				return state, nil
			}
		}
		// Read errors (transient 5xx, rate limits already retried by the
		// transport) are not fatal: keep polling until ctx/timeout.
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-budget:
			return "", ErrWaitTimedOut
		case <-ticker.C:
		}
	}
}

// initWait normalizes the wait parameters: it clamps the poll interval,
// arms the wait budget, and builds the wanted-contexts set. The budget
// is a plain timer channel, not context.WithTimeout: our own expiry must
// surface as ErrWaitTimedOut while a caller's context cancellation
// surfaces as ctx.Err() — the two are different diagnoses and callers
// check for them differently.
func initWait(opts WaitOptions) (interval time.Duration, budget <-chan time.Time, want map[string]bool) {
	interval = opts.Interval
	if interval <= 0 {
		interval = DefaultWaitInterval
	}
	if interval < minWaitInterval {
		interval = minWaitInterval
	}
	if opts.Timeout > 0 {
		// The timer is intentionally never stopped: the budget must stay
		// armed for the whole (unbounded) wait loop, and an unstopped
		// timer keeps firing regardless of when the wrapper is collected.
		t := time.NewTimer(opts.Timeout)
		budget = t.C
	}
	want = make(map[string]bool, len(opts.Contexts))
	for _, c := range opts.Contexts {
		want[c] = true
	}
	return interval, budget, want
}

// foldTerminalStates combines the statuses filtered to the wanted
// contexts and reports whether the combined state is final. With a
// context filter, absence of ANY wanted context means the fold cannot be
// final yet — it is reported as pending regardless of what landed so far.
func foldTerminalStates(statuses []CommitStatus, want map[string]bool) (CommitStatusState, bool) {
	states := make([]CommitStatusState, 0, len(statuses))
	for _, s := range statuses {
		if len(want) > 0 && !want[s.Context] {
			continue
		}
		states = append(states, s.State)
	}
	combined := CombineCommitStatusStates(states)
	if len(want) > 0 && combined.Terminal() && len(states) < len(want) {
		return CommitStatusPending, false
	}
	return combined, combined.Terminal()
}

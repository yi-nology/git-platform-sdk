package backendutil

import "log"

// maxAllPages bounds AllPages as a safety net against platforms that do not
// advance pagination. Observed in the field: a Forgejo 15.0.1 instance ignored
// the page parameter on the issue-comment list endpoint and returned the
// identical full page for every page number, so the "stop on first empty page"
// rule never fired and callers spun forever. maxAllPages×pageSize items is far
// beyond any real list; hitting the cap is logged loudly because it almost
// certainly means data truncation caused by a platform pagination bug.
const maxAllPages = 50

// AllPages fetches every page of a paginated list by advancing the page
// number until the platform returns an empty page. fetch receives the
// 1-based page number and must request that page with whatever page size
// the caller chose. Stopping on the first empty page (rather than on a
// short page) keeps the result complete even when the server caps the page
// size below the requested per-page value; the platform's list endpoint
// must honor the page parameter, which every supported platform's list API
// does.
func AllPages[T any](fetch func(page int) ([]T, error)) ([]T, error) {
	var all []T
	for page := 1; ; page++ {
		if page > maxAllPages {
			log.Printf("backendutil.AllPages: hit %d-page cap; the platform list endpoint likely ignores the page parameter — result truncated", maxAllPages)
			return all, nil
		}
		batch, err := fetch(page)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return all, nil
		}
		all = append(all, batch...)
	}
}

package backendutil

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

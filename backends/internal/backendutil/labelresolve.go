package backendutil

import "errors"

// ErrLabelScanLimit reports that the named label was not found within the
// scanned page budget. It is distinct from a definitive not-found: the label
// may exist beyond the limit, so callers surface it as a scan-limit error
// rather than a 404.
var ErrLabelScanLimit = errors.New("label not found within scan limit")

// LabelRef is the minimal label shape the resolver needs from one platform
// page fetch.
type LabelRef struct {
	ID   int64
	Name string
}

// LabelPageFunc fetches one page of labels; page is 1-based, perPage is the
// requested page size, and a page returning fewer than perPage refs ends the
// scan.
type LabelPageFunc func(page, perPage int) ([]LabelRef, error)

// ResolveLabelID scans pages for the named label and returns its ID, or
// ErrLabelScanLimit when the budget is exhausted or a short page is reached
// without a match. Scan errors propagate unchanged.
func ResolveLabelID(scan LabelPageFunc, name string, maxPages, perPage int) (int64, error) {
	for page := 1; page <= maxPages; page++ {
		refs, err := scan(page, perPage)
		if err != nil {
			return 0, err
		}
		for _, r := range refs {
			if r.Name == name {
				return r.ID, nil
			}
		}
		if len(refs) < perPage {
			break
		}
	}
	return 0, ErrLabelScanLimit
}

// ResolveLabel resolves via ResolveLabelID and caches the result under
// key+"/"+name. Failures are not cached.
func (c *IDCache) ResolveLabel(key, name string, scan LabelPageFunc, maxPages, perPage int) (int64, error) {
	if id, ok := c.Get(key + "/" + name); ok {
		return id, nil
	}
	id, err := ResolveLabelID(scan, name, maxPages, perPage)
	if err != nil {
		return 0, err
	}
	c.Put(key+"/"+name, id)
	return id, nil
}

// ScanLimitError wraps ErrLabelScanLimit with a platform-specific message
// (errors.Is reports it as ErrLabelScanLimit). Callers that need to
// distinguish "not found within the scan budget" from other failures use
// errors.Is(err, backendutil.ErrLabelScanLimit).
type ScanLimitError struct {
	Msg string
}

func (e *ScanLimitError) Error() string { return e.Msg }
func (e *ScanLimitError) Unwrap() error { return ErrLabelScanLimit }

// NewScanLimitError builds a ScanLimitError with the given message.
func NewScanLimitError(msg string) error { return &ScanLimitError{Msg: msg} }

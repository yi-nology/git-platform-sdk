package backendutil

import (
	"errors"
	"testing"
	"time"
)

func TestResolveLabelIDFindsOnSecondPage(t *testing.T) {
	pages := map[int][]LabelRef{
		1: {{ID: 1, Name: "a"}, {ID: 2, Name: "b"}},
		2: {{ID: 3, Name: "target"}},
	}
	id, err := ResolveLabelID(func(page, perPage int) ([]LabelRef, error) {
		return pages[page], nil
	}, "target", 5, 2)
	if err != nil || id != 3 {
		t.Fatalf("ResolveLabelID = (%d, %v), want (3, nil)", id, err)
	}
}

func TestResolveLabelIDStopsAtShortPage(t *testing.T) {
	calls := 0
	_, err := ResolveLabelID(func(page, perPage int) ([]LabelRef, error) {
		calls++
		return []LabelRef{{ID: 1, Name: "a"}}, nil // short page: no more pages
	}, "missing", 50, 100)
	if !errors.Is(err, ErrLabelScanLimit) {
		t.Fatalf("err = %v, want ErrLabelScanLimit", err)
	}
	if calls != 1 {
		t.Fatalf("scan calls = %d, want 1 (short page must stop the scan)", calls)
	}
}

func TestResolveLabelIDScanLimitDistinctFromShortPage(t *testing.T) {
	_, err := ResolveLabelID(func(page, perPage int) ([]LabelRef, error) {
		return []LabelRef{{ID: int64(page), Name: "other"}}, nil // always full page? no: 1 < 100
	}, "missing", 50, 100)
	// 1 result < perPage 100 → short page → stop → scan limit error, 1 call.
	if !errors.Is(err, ErrLabelScanLimit) {
		t.Fatalf("err = %v, want ErrLabelScanLimit", err)
	}
}

func TestIDCacheResolveLabelCaches(t *testing.T) {
	c := NewIDCache(time.Minute)
	scans := 0
	scan := func(page, perPage int) ([]LabelRef, error) {
		scans++
		return []LabelRef{{ID: 7, Name: "x"}}, nil
	}
	for i := 0; i < 3; i++ {
		id, err := c.ResolveLabel("owner/repo", "x", scan, 50, 100)
		if err != nil || id != 7 {
			t.Fatalf("ResolveLabel #%d = (%d, %v)", i, id, err)
		}
	}
	if scans != 1 {
		t.Fatalf("scans = %d, want 1 (second and third resolve must hit the cache)", scans)
	}
}

func TestNewScanLimitErrorPreservesSentinel(t *testing.T) {
	const msg = `label "missing" not found within 50 pages (scan limit)`
	err := NewScanLimitError(msg)
	if !errors.Is(err, ErrLabelScanLimit) {
		t.Fatalf("errors.Is(err, ErrLabelScanLimit) = false for %v, want true", err)
	}
	if err.Error() != msg {
		t.Fatalf("err.Error() = %q, want %q", err.Error(), msg)
	}
}

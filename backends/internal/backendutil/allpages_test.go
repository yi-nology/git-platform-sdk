package backendutil

import (
	"errors"
	"slices"
	"testing"
)

// TestAllPagesMergesUntilEmpty verifies the loop advances pages and stops
// on the first empty page, merging everything seen along the way. Stopping
// on empty (not on "short page") keeps the result complete even when the
// server caps the page size below the requested per-page value.
func TestAllPagesMergesUntilEmpty(t *testing.T) {
	pages := [][]int{{1, 2, 3}, {4}, {}}
	var fetched []int
	got, err := AllPages(func(page int) ([]int, error) {
		if page > len(pages) {
			t.Fatalf("fetched past the empty terminating page: %d", page)
		}
		fetched = append(fetched, page)
		return pages[page-1], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []int{1, 2, 3, 4}
	if !slices.Equal(got, want) {
		t.Errorf("AllPages = %v, want %v", got, want)
	}
	if !slices.Equal(fetched, []int{1, 2, 3}) {
		t.Errorf("fetched pages %v, want [1 2 3] (stop on the empty page)", fetched)
	}
}

// TestAllPagesSingleEmptyPage verifies an empty first page yields an empty
// result without a second fetch.
func TestAllPagesSingleEmptyPage(t *testing.T) {
	fetches := 0
	got, err := AllPages(func(page int) ([]string, error) {
		fetches++
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || fetches != 1 {
		t.Errorf("got %d items in %d fetches, want 0 items in 1 fetch", len(got), fetches)
	}
}

// TestAllPagesPropagatesErrors verifies a failing fetch aborts the loop
// with the error surfaced.
func TestAllPagesPropagatesErrors(t *testing.T) {
	want := errors.New("boom")
	_, err := AllPages(func(page int) ([]int, error) {
		if page == 2 {
			return nil, want
		}
		return []int{1}, nil
	})
	if !errors.Is(err, want) {
		t.Errorf("expected the fetch error to surface, got %v", err)
	}
}

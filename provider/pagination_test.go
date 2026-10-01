package provider

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestNormalizePageOpts_Defaults(t *testing.T) {
	page, perPage := NormalizePageOpts(0, 0)
	if page != DefaultPage {
		t.Fatalf("expected page %d, got %d", DefaultPage, page)
	}
	if perPage != DefaultPerPage {
		t.Fatalf("expected perPage %d, got %d", DefaultPerPage, perPage)
	}
}

func TestNormalizePageOpts_Values(t *testing.T) {
	page, perPage := NormalizePageOpts(3, 50)
	if page != 3 {
		t.Fatalf("expected 3, got %d", page)
	}
	if perPage != 50 {
		t.Fatalf("expected 50, got %d", perPage)
	}
}

func TestNormalizePageOpts_MaxPerPage(t *testing.T) {
	_, perPage := NormalizePageOpts(1, 200)
	if perPage != MaxPerPage {
		t.Fatalf("expected %d, got %d", MaxPerPage, perPage)
	}
}

func TestNormalizePageOpts_NegativeValues(t *testing.T) {
	page, perPage := NormalizePageOpts(-1, -5)
	if page != DefaultPage {
		t.Fatalf("expected page %d, got %d", DefaultPage, page)
	}
	if perPage != DefaultPerPage {
		t.Fatalf("expected perPage %d, got %d", DefaultPerPage, perPage)
	}
}

func TestParseTotalCountHeader_XTotalCount(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Total-Count", "42")
	result := ParseTotalCountHeader(headers, 0)
	if result != 42 {
		t.Fatalf("expected 42, got %d", result)
	}
}

func TestParseTotalCountHeader_XTotal(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Total", "100")
	result := ParseTotalCountHeader(headers, 0)
	if result != 100 {
		t.Fatalf("expected 100, got %d", result)
	}
}

func TestParseTotalCountHeader_Fallback(t *testing.T) {
	headers := http.Header{}
	result := ParseTotalCountHeader(headers, 10)
	if result != 10 {
		t.Fatalf("expected 10, got %d", result)
	}
}

func TestParseTotalCountHeader_InvalidValue(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Total-Count", "abc")
	result := ParseTotalCountHeader(headers, 5)
	if result != 5 {
		t.Fatalf("expected 5, got %d", result)
	}
}

// --- ListAllPages ---

func TestListAllPages_CollectsUntilShortPage(t *testing.T) {
	// 3 full pages of 10 plus a final partial page of 4.
	fetch := func(_ context.Context, page, perPage int) ([]int, error) {
		if page > 4 {
			t.Fatalf("must stop after the short page, but page %d was requested", page)
		}
		size := perPage
		if page == 4 {
			size = 4
		}
		out := make([]int, size)
		for i := range out {
			out[i] = (page-1)*perPage + i
		}
		return out, nil
	}
	got, err := ListAllPages[int](context.Background(), 10, 0, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 34 {
		t.Fatalf("expected 34 items (3*10+4), got %d", len(got))
	}
	for i, v := range got {
		if v != i {
			t.Fatalf("items out of order at index %d: got %d", i, v)
		}
	}
}

func TestListAllPages_StopsOnFirstShortPage(t *testing.T) {
	// First page already short: exactly one fetch, no further pages.
	var calls int
	fetch := func(_ context.Context, page, _ int) ([]int, error) {
		calls++
		if page != 1 {
			t.Fatalf("expected stop after page 1, but page %d fetched", page)
		}
		return []int{1, 2}, nil
	}
	got, err := ListAllPages[int](context.Background(), 10, 0, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || calls != 1 {
		t.Fatalf("expected 2 items from 1 fetch, got %d items / %d calls", len(got), calls)
	}
}

func TestListAllPages_MaxPagesSafetyCap(t *testing.T) {
	// Server always returns a full page (simulates a platform that ignores
	// the page parameter): the loop must stop at maxPages instead of
	// spinning forever.
	var calls int
	fetch := func(_ context.Context, _, perPage int) ([]int, error) {
		calls++
		return make([]int, perPage), nil
	}
	got, err := ListAllPages[int](context.Background(), 5, 3, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("expected exactly maxPages=3 fetches, got %d", calls)
	}
	if len(got) != 15 {
		t.Fatalf("expected 15 items (3 pages * 5), got %d", len(got))
	}
}

func TestListAllPages_DefaultPerPageAndMaxPages(t *testing.T) {
	// perPage<=0 falls back to MaxPerPage: the fetch must receive 100.
	// maxPages<=0 falls back to 100: a server returning full pages forever
	// must be capped at 100 fetches.
	var gotPerPage, calls int
	fetch := func(_ context.Context, _ int, perPage int) ([]int, error) {
		gotPerPage = perPage
		calls++
		return make([]int, perPage), nil
	}
	got, err := ListAllPages[int](context.Background(), 0, 0, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if gotPerPage != MaxPerPage {
		t.Fatalf("expected default perPage %d, fetch got %d", MaxPerPage, gotPerPage)
	}
	if calls != 100 {
		t.Fatalf("expected default maxPages=100 fetches, got %d", calls)
	}
	if len(got) != 100*MaxPerPage {
		t.Fatalf("expected %d items, got %d", 100*MaxPerPage, len(got))
	}
}

func TestListAllPages_FetchErrorPropagates(t *testing.T) {
	sentinel := errors.New("boom")
	var calls int
	fetch := func(_ context.Context, page, _ int) ([]int, error) {
		calls++
		if page == 2 {
			return nil, sentinel
		}
		return make([]int, 10), nil
	}
	got, err := ListAllPages[int](context.Background(), 10, 0, fetch)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil result on error, got %v", got)
	}
	if calls != 2 {
		t.Fatalf("expected stop at failing page (2 calls), got %d", calls)
	}
}

func TestListAllPages_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls int
	fetch := func(_ context.Context, _, _ int) ([]int, error) {
		calls++
		return make([]int, 10), nil
	}
	_, err := ListAllPages[int](ctx, 10, 0, fetch)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("canceled ctx must abort before any fetch, got %d calls", calls)
	}

	// Cancel mid-iteration: the next loop pass must observe the cancellation.
	ctx2, cancel2 := context.WithCancel(context.Background())
	calls = 0
	fetch2 := func(_ context.Context, _, _ int) ([]int, error) {
		calls++
		cancel2()
		return make([]int, 10), nil
	}
	_, err = ListAllPages[int](ctx2, 10, 0, fetch2)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled mid-run, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 fetch before aborting, got %d", calls)
	}
}

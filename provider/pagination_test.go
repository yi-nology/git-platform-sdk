package provider

import (
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

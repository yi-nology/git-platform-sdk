package gitbackend

import (
	"reflect"
	"testing"
)

func TestMergeInsecure(t *testing.T) {
	t.Run("explicit flag sets insecure on returned auth", func(t *testing.T) {
		a := mergeInsecure(AuthConfig{Type: AuthHTTPToken, Token: "t"}, true)
		if !a.InsecureSkipTLS {
			t.Fatal("expected InsecureSkipTLS=true after merge")
		}
		if a.Token != "t" {
			t.Fatalf("expected token preserved, got %q", a.Token)
		}
	})

	t.Run("false flag preserves auth.InsecureSkipTLS", func(t *testing.T) {
		a := mergeInsecure(AuthConfig{InsecureSkipTLS: true}, false)
		if !a.InsecureSkipTLS {
			t.Fatal("expected pre-existing InsecureSkipTLS to be preserved")
		}
	})

	t.Run("neither set stays false", func(t *testing.T) {
		a := mergeInsecure(AuthConfig{}, false)
		if a.InsecureSkipTLS {
			t.Fatal("expected InsecureSkipTLS=false")
		}
	})

	t.Run("returned auth is a copy", func(t *testing.T) {
		orig := AuthConfig{Token: "x"}
		got := mergeInsecure(orig, true)
		got.Token = "changed"
		if orig.Token != "x" {
			t.Fatal("mergeInsecure should not mutate the input auth")
		}
	})
}

func TestWithInsecureArgs(t *testing.T) {
	base := []string{"fetch", "origin"}

	t.Run("insecure prepends -c http.sslVerify=false", func(t *testing.T) {
		got := withInsecureArgs(AuthConfig{InsecureSkipTLS: true}, base)
		want := []string{"-c", "http.sslVerify=false", "fetch", "origin"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("secure leaves args untouched", func(t *testing.T) {
		got := withInsecureArgs(AuthConfig{}, base)
		if !reflect.DeepEqual(got, base) {
			t.Fatalf("got %v, want %v", got, base)
		}
		// ensure the original slice was not mutated
		if !reflect.DeepEqual(base, []string{"fetch", "origin"}) {
			t.Fatal("withInsecureArgs mutated the input slice")
		}
	})
}

func TestOpenRepositoryCarriesInsecure(t *testing.T) {
	t.Run("insecure=true is carried on auth", func(t *testing.T) {
		r := OpenRepository(nil, "/tmp/repo", AuthConfig{Type: AuthHTTPToken, Token: "t"}, true)
		if !r.Auth().InsecureSkipTLS {
			t.Fatal("expected Repository.Auth().InsecureSkipTLS=true")
		}
	})

	t.Run("insecure=false keeps auth untouched", func(t *testing.T) {
		r := OpenRepository(nil, "/tmp/repo", AuthConfig{Type: AuthHTTPToken}, false)
		if r.Auth().InsecureSkipTLS {
			t.Fatal("expected Repository.Auth().InsecureSkipTLS=false")
		}
	})
}

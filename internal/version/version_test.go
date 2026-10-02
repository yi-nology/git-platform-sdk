package version

import (
	"strings"
	"testing"
)

func TestStringNonEmpty(t *testing.T) {
	v := String()
	if v == "" {
		t.Fatal("String() must never be empty")
	}
	if strings.Contains(v, " ") {
		t.Fatalf("String() = %q, versions must not contain spaces", v)
	}
}

func TestModulePath(t *testing.T) {
	if !strings.Contains(ModulePath, "go-git-platform") {
		t.Fatalf("ModulePath = %q", ModulePath)
	}
}

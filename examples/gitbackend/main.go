// Example: use the local git backend to init a repository in a temp dir and
// list its branches. No network access required.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yi-nology/git-platform-sdk/gitbackend"
)

func main() {
	backend, err := gitbackend.NewGitBackend(gitbackend.Options{Type: "native"})
	if err != nil {
		log.Fatalf("new git backend: %v", err)
	}

	dir, err := os.MkdirTemp("", "gps-example-*")
	if err != nil {
		log.Fatalf("scratch dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	ctx := context.Background()
	if err := backend.Init(ctx, dir); err != nil {
		log.Fatalf("init: %v", err)
	}

	branches, err := backend.ListLocalBranches(ctx, dir)
	if err != nil {
		log.Fatalf("list branches: %v", err)
	}
	fmt.Println("local branches:", branches)
}

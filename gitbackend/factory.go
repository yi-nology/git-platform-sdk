package gitbackend

import "fmt"

func NewGitBackend(backendType string) (GitBackend, error) {
	switch backendType {
	case "native":
		backend, err := NewNativeGitBackend()
		if err != nil {
			return nil, fmt.Errorf("native git backend unavailable, falling back: %w", err)
		}
		return backend, nil
	case "gogit":
		return NewGoGitBackend(), nil
	default:
		backend, err := NewNativeGitBackend()
		if err != nil {
			return NewGoGitBackend(), nil
		}
		return backend, nil
	}
}

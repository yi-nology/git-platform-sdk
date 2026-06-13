package provider

import (
	"strings"
)

// SplitFullName splits "owner/repo" into (owner, repo).
// If the input doesn't contain "/", owner is empty.
func SplitFullName(fullName string) (owner, name string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", fullName
}

// ExtractOwnerFromFullName returns just the owner portion of "owner/repo".
func ExtractOwnerFromFullName(fullName string) string {
	owner, _ := SplitFullName(fullName)
	return owner
}

// BuildEventRepo creates an EventRepo from a full name string.
func BuildEventRepo(fullName string) *EventRepo {
	owner, name := SplitFullName(fullName)
	return &EventRepo{
		FullName: fullName,
		Owner:    owner,
		Name:     name,
	}
}

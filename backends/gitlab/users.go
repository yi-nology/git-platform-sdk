package gitlab

import (
	"context"
	"fmt"
	"net/http"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// resolveUserIDs resolves usernames to GitLab user IDs using ListUsers with
// the exact-match username filter, memoized in a per-provider TTL cache.
// op is the public operation the resolution serves; an unknown username
// surfaces as a NotFound under that op.
func (p *Provider) resolveUserIDs(ctx context.Context, op string, usernames []string) ([]int64, error) {
	ids := make([]int64, 0, len(usernames))
	for _, name := range usernames {
		if id, ok := p.userIDs.Get(name); ok {
			ids = append(ids, id)
			continue
		}
		users, _, err := p.client.Users.ListUsers(
			&gitlab.ListUsersOptions{Username: gitlab.Ptr(name)},
			gitlab.WithContext(ctx))
		if err != nil {
			return nil, provider.Wrap(provider.PlatformGitLab, op, err)
		}
		var (
			id    int64
			found bool
		)
		for _, u := range users {
			if u != nil && u.Username == name {
				id, found = u.ID, true
				break
			}
		}
		if !found {
			return nil, provider.New(provider.PlatformGitLab, op, http.StatusNotFound, fmt.Sprintf("user %q not found", name))
		}
		p.userIDs.Put(name, id)
		ids = append(ids, id)
	}
	return ids, nil
}

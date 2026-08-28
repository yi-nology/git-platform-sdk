package tencentcode

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// resolveUserIDs resolves usernames to 工蜂 user IDs through the Users API
// (GET /users/{username} — the endpoint accepts either an ID or a username),
// memoized in a per-provider TTL cache. op is the public operation the
// resolution serves; an unknown username surfaces as a NotFound under that
// op, and resolution failures stop the write before it reaches the wire.
func (p *Provider) resolveUserIDs(ctx context.Context, op string, usernames []string) ([]int, error) {
	ids := make([]int, 0, len(usernames))
	for _, name := range usernames {
		if id, ok := p.userIDs.Get(name); ok {
			ids = append(ids, int(id))
			continue
		}
		u, _, err := p.client.Users.GetUser(ctx, name)
		if err != nil {
			// Wrap classifies a 404 lookup as NotFound, so an unknown
			// username keeps the unified "user not found" shape.
			return nil, sdkError(op, err)
		}
		if u == nil || u.ID == 0 {
			return nil, provider.New(provider.PlatformTencentCode, op, http.StatusNotFound,
				"user "+strconv.Quote(name)+" not found")
		}
		p.userIDs.Put(name, int64(u.ID))
		ids = append(ids, u.ID)
	}
	return ids, nil
}

// assigneeIDsCSV renders resolved user IDs as the csv string 工蜂's
// assignee_ids field carries on the wire.
func assigneeIDsCSV(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}

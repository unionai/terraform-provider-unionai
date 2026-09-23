package provider

import (
	"context"
	"sync"

	"github.com/unionai/cloud/gen/pb-go/authorizer"
	"github.com/unionai/cloud/gen/pb-go/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// identityAssignmentCache memoizes GetIdentityAssignments per identity for the lifetime of the
// provider instance. Access resources are keyed by (identity, policy), so without it refreshing N
// bindings for one identity issues N identical calls. Concurrent lookups for the same identity
// share a single in-flight request. Successful and NotFound responses are cached; other errors
// are not, so a transient failure is retried by the next lookup.
type identityAssignmentCache struct {
	mu      sync.Mutex
	entries map[string]*identityAssignmentEntry
}

type identityAssignmentEntry struct {
	done chan struct{}
	resp *authorizer.GetIdentityAssignmentResponse
	err  error
}

func newIdentityAssignmentCache() *identityAssignmentCache {
	return &identityAssignmentCache{entries: map[string]*identityAssignmentEntry{}}
}

func identityAssignmentKey(org string, identity *common.Identity) string {
	b, _ := proto.MarshalOptions{Deterministic: true}.Marshal(identity)
	return org + "/" + string(b)
}

// get returns the identity's assignments, calling conn only if they are not already cached.
// A nil cache always calls conn.
func (c *identityAssignmentCache) get(ctx context.Context, conn authorizer.AuthorizerServiceClient, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
	if c == nil {
		return conn.GetIdentityAssignments(ctx, req)
	}

	key := identityAssignmentKey(req.Organization, req.Identity)
	c.mu.Lock()
	e, ok := c.entries[key]
	if !ok {
		e = &identityAssignmentEntry{done: make(chan struct{})}
		c.entries[key] = e
		c.mu.Unlock()

		e.resp, e.err = conn.GetIdentityAssignments(ctx, req)
		if e.err != nil && status.Code(e.err) != codes.NotFound {
			c.mu.Lock()
			if c.entries[key] == e {
				delete(c.entries, key)
			}
			c.mu.Unlock()
		}
		close(e.done)
		return e.resp, e.err
	}
	c.mu.Unlock()

	select {
	case <-e.done:
		return e.resp, e.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// invalidate drops the cached assignments for an identity whose assignments were just changed.
func (c *identityAssignmentCache) invalidate(org string, identity *common.Identity) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.entries, identityAssignmentKey(org, identity))
	c.mu.Unlock()
}

// hasPolicy reports whether resp includes the given policy in org.
func hasPolicy(resp *authorizer.GetIdentityAssignmentResponse, org, policy string) bool {
	for _, p := range resp.GetIdentityAssignment().GetPolicies() {
		if p.GetId().GetName() == policy && p.GetId().GetOrganization() == org {
			return true
		}
	}
	return false
}

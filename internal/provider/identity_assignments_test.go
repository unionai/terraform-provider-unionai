package provider

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/unionai/cloud/gen/pb-go/authorizer"
	"github.com/unionai/cloud/gen/pb-go/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func assignmentsWithPolicies(org string, policies ...string) *authorizer.GetIdentityAssignmentResponse {
	resp := &authorizer.GetIdentityAssignmentResponse{IdentityAssignment: &authorizer.IdentityAssignment{}}
	for _, p := range policies {
		resp.IdentityAssignment.Policies = append(resp.IdentityAssignment.Policies,
			&common.Policy{Id: &common.PolicyIdentifier{Name: p, Organization: org}})
	}
	return resp
}

func assignmentRequest(identity *common.Identity) *authorizer.GetIdentityAssignmentRequest {
	return &authorizer.GetIdentityAssignmentRequest{Organization: "test-org", Identity: identity}
}

func TestIdentityAssignmentCache_ConcurrentLookupsShareOneCall(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			calls.Add(1)
			<-release
			return assignmentsWithPolicies("test-org", "p1"), nil
		},
	}

	c := newIdentityAssignmentCache()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := c.get(context.Background(), mock, assignmentRequest(userIdentity("alice")))
			if err != nil || !hasPolicy(resp, "test-org", "p1") {
				t.Errorf("get() = %v, %v", resp, err)
			}
		}()
	}
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("Expected 1 GetIdentityAssignments call, got %d", got)
	}
}

func TestIdentityAssignmentCache_KeysByIdentity(t *testing.T) {
	var calls atomic.Int32
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			calls.Add(1)
			return assignmentsWithPolicies("test-org"), nil
		},
	}

	c := newIdentityAssignmentCache()
	for _, id := range []*common.Identity{userIdentity("alice"), userIdentity("bob"), appIdentity("alice"), userIdentity("alice")} {
		if _, err := c.get(context.Background(), mock, assignmentRequest(id)); err != nil {
			t.Fatalf("get() returned error: %v", err)
		}
	}

	if got := calls.Load(); got != 3 {
		t.Errorf("Expected 3 GetIdentityAssignments calls (user alice, user bob, app alice), got %d", got)
	}
}

func TestIdentityAssignmentCache_CachesNotFound(t *testing.T) {
	var calls atomic.Int32
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			calls.Add(1)
			return nil, status.Error(codes.NotFound, "not found")
		},
	}

	c := newIdentityAssignmentCache()
	for i := 0; i < 2; i++ {
		if _, err := c.get(context.Background(), mock, assignmentRequest(userIdentity("alice"))); status.Code(err) != codes.NotFound {
			t.Fatalf("Expected NotFound, got %v", err)
		}
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("Expected NotFound to be cached (1 call), got %d", got)
	}
}

func TestIdentityAssignmentCache_DoesNotCacheTransientErrors(t *testing.T) {
	var calls atomic.Int32
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			if calls.Add(1) == 1 {
				return nil, status.Error(codes.Unavailable, "try again")
			}
			return assignmentsWithPolicies("test-org", "p1"), nil
		},
	}

	c := newIdentityAssignmentCache()
	if _, err := c.get(context.Background(), mock, assignmentRequest(userIdentity("alice"))); status.Code(err) != codes.Unavailable {
		t.Fatalf("Expected Unavailable, got %v", err)
	}
	resp, err := c.get(context.Background(), mock, assignmentRequest(userIdentity("alice")))
	if err != nil || !hasPolicy(resp, "test-org", "p1") {
		t.Fatalf("Expected retry to succeed, got %v, %v", resp, err)
	}

	if got := calls.Load(); got != 2 {
		t.Errorf("Expected 2 calls, got %d", got)
	}
}

func TestIdentityAssignmentCache_Invalidate(t *testing.T) {
	var calls atomic.Int32
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			calls.Add(1)
			return assignmentsWithPolicies("test-org"), nil
		},
	}

	c := newIdentityAssignmentCache()
	req := assignmentRequest(userIdentity("alice"))
	_, _ = c.get(context.Background(), mock, req)
	c.invalidate("test-org", userIdentity("alice"))
	_, _ = c.get(context.Background(), mock, req)

	if got := calls.Load(); got != 2 {
		t.Errorf("Expected invalidate to force a second call, got %d calls", got)
	}
}

func TestIdentityAssignmentCache_NilCacheCallsThrough(t *testing.T) {
	var calls atomic.Int32
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			calls.Add(1)
			return assignmentsWithPolicies("test-org"), nil
		},
	}

	var c *identityAssignmentCache
	_, _ = c.get(context.Background(), mock, assignmentRequest(userIdentity("alice")))
	_, _ = c.get(context.Background(), mock, assignmentRequest(userIdentity("alice")))
	c.invalidate("test-org", userIdentity("alice"))

	if got := calls.Load(); got != 2 {
		t.Errorf("Expected nil cache to call through every time, got %d calls", got)
	}
}

func TestHasPolicy(t *testing.T) {
	resp := assignmentsWithPolicies("test-org", "p1")
	if !hasPolicy(resp, "test-org", "p1") {
		t.Error("Expected p1 in test-org")
	}
	if hasPolicy(resp, "other-org", "p1") {
		t.Error("Expected org mismatch to be false")
	}
	if hasPolicy(resp, "test-org", "p2") {
		t.Error("Expected p2 to be absent")
	}
	if hasPolicy(&authorizer.GetIdentityAssignmentResponse{}, "test-org", "p1") {
		t.Error("Expected empty response to be false")
	}
	if hasPolicy(nil, "test-org", "p1") {
		t.Error("Expected nil response to be false")
	}
}

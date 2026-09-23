package provider

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/unionai/cloud/gen/pb-go/authorizer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newUserAccessState(t *testing.T, user, policy string) tfsdk.State {
	t.Helper()
	s := &resource.SchemaResponse{}
	NewUserAccessResource().Schema(context.Background(), resource.SchemaRequest{}, s)
	if s.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", s.Diagnostics.Errors())
	}
	return tfsdk.State{
		Schema: s.Schema,
		Raw: tftypes.NewValue(tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"user":   tftypes.String,
				"policy": tftypes.String,
			},
		}, map[string]tftypes.Value{
			"user":   tftypes.NewValue(tftypes.String, user),
			"policy": tftypes.NewValue(tftypes.String, policy),
		}),
	}
}

func readUserAccess(t *testing.T, r *UserAccessResource, user, policy string) *resource.ReadResponse {
	t.Helper()
	state := newUserAccessState(t, user, policy)
	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read() returned errors: %v", resp.Diagnostics.Errors())
	}
	return resp
}

func TestUserAccessResource_Read_Assigned(t *testing.T) {
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			if req.Identity.GetUserId().GetSubject() != "alice" {
				t.Errorf("Expected user 'alice' in read request, got %v", req.Identity)
			}
			return assignmentsWithPolicies("test-org", "my-policy"), nil
		},
	}
	r := &UserAccessResource{conn: mock, org: "test-org"}

	resp := readUserAccess(t, r, "alice", "my-policy")

	var data UserAccessResourceModel
	resp.State.Get(context.Background(), &data)
	if data.User.ValueString() != "alice" || data.Policy.ValueString() != "my-policy" {
		t.Errorf("Expected state preserved, got %+v", data)
	}
}

func TestUserAccessResource_Read_PolicyNoLongerAssigned(t *testing.T) {
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			return assignmentsWithPolicies("test-org", "other-policy"), nil
		},
	}
	r := &UserAccessResource{conn: mock, org: "test-org"}

	if resp := readUserAccess(t, r, "alice", "my-policy"); !resp.State.Raw.IsNull() {
		t.Error("Expected state to be removed when the policy is no longer assigned")
	}
}

func TestUserAccessResource_Read_NotFound(t *testing.T) {
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			return nil, status.Error(codes.NotFound, "not found")
		},
	}
	r := &UserAccessResource{conn: mock, org: "test-org"}

	if resp := readUserAccess(t, r, "alice", "my-policy"); !resp.State.Raw.IsNull() {
		t.Error("Expected state to be removed on NotFound")
	}
}

func TestUserAccessResource_Read_SharesLookupAcrossBindings(t *testing.T) {
	var calls atomic.Int32
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			calls.Add(1)
			return assignmentsWithPolicies("test-org", "p1", "p2", "p3"), nil
		},
	}
	cache := newIdentityAssignmentCache()

	for _, policy := range []string{"p1", "p2", "p3"} {
		r := &UserAccessResource{conn: mock, org: "test-org", assignments: cache}
		if resp := readUserAccess(t, r, "alice", policy); resp.State.Raw.IsNull() {
			t.Errorf("Expected binding to %s to be kept", policy)
		}
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("Expected 1 GetIdentityAssignments call for 3 bindings of one user, got %d", got)
	}
}

func TestUserAccessResource_Create_InvalidatesCache(t *testing.T) {
	var calls atomic.Int32
	mock := &mockAuthorizerClient{
		assignFn: func(ctx context.Context, req *authorizer.AssignIdentityRequest) (*authorizer.AssignIdentityResponse, error) {
			return &authorizer.AssignIdentityResponse{}, nil
		},
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			if calls.Add(1) == 1 {
				return assignmentsWithPolicies("test-org"), nil
			}
			return assignmentsWithPolicies("test-org", "my-policy"), nil
		},
	}
	r := &UserAccessResource{conn: mock, org: "test-org", assignments: newIdentityAssignmentCache()}

	if resp := readUserAccess(t, r, "alice", "my-policy"); !resp.State.Raw.IsNull() {
		t.Fatal("Expected binding to be absent before Create")
	}

	state := newUserAccessState(t, "alice", "my-policy")
	createResp := &resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: state.Schema, Raw: state.Raw}}, createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("Create() returned errors: %v", createResp.Diagnostics.Errors())
	}

	if resp := readUserAccess(t, r, "alice", "my-policy"); resp.State.Raw.IsNull() {
		t.Error("Expected Read after Create to see the new assignment")
	}
}

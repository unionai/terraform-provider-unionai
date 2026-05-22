package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/unionai/cloud/gen/pb-go/authorizer"
	"github.com/unionai/cloud/gen/pb-go/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockAuthorizerClient implements the subset of AuthorizerServiceClient used by AppAccessResource.
type mockAuthorizerClient struct {
	authorizer.AuthorizerServiceClient
	assignFn     func(ctx context.Context, req *authorizer.AssignIdentityRequest) (*authorizer.AssignIdentityResponse, error)
	unassignFn   func(ctx context.Context, req *authorizer.UnassignIdentityRequest) (*authorizer.UnassignIdentityResponse, error)
	getAssignFn  func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error)
}

func (m *mockAuthorizerClient) AssignIdentity(ctx context.Context, in *authorizer.AssignIdentityRequest, opts ...grpc.CallOption) (*authorizer.AssignIdentityResponse, error) {
	return m.assignFn(ctx, in)
}

func (m *mockAuthorizerClient) UnassignIdentity(ctx context.Context, in *authorizer.UnassignIdentityRequest, opts ...grpc.CallOption) (*authorizer.UnassignIdentityResponse, error) {
	return m.unassignFn(ctx, in)
}

func (m *mockAuthorizerClient) GetIdentityAssignments(ctx context.Context, in *authorizer.GetIdentityAssignmentRequest, opts ...grpc.CallOption) (*authorizer.GetIdentityAssignmentResponse, error) {
	return m.getAssignFn(ctx, in)
}

func TestAppAccessResource_Metadata(t *testing.T) {
	r := NewAppAccessResource()
	req := resource.MetadataRequest{ProviderTypeName: "unionai"}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), req, resp)

	if resp.TypeName != "unionai_application_access" {
		t.Errorf("Expected type name 'unionai_application_access', got '%s'", resp.TypeName)
	}
}

func TestAppAccessResource_Schema(t *testing.T) {
	r := NewAppAccessResource()
	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics.Errors())
	}

	if _, ok := resp.Schema.Attributes["app"]; !ok {
		t.Error("Expected 'app' attribute in schema")
	}
	if _, ok := resp.Schema.Attributes["policy"]; !ok {
		t.Error("Expected 'policy' attribute in schema")
	}
}

func newTestState(t *testing.T, app, policy string) tfsdk.State {
	t.Helper()
	s := testAppAccessSchema(t)
	return tfsdk.State{
		Schema: s.Schema,
		Raw: tftypes.NewValue(tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"app":    tftypes.String,
				"policy": tftypes.String,
			},
		}, map[string]tftypes.Value{
			"app":    tftypes.NewValue(tftypes.String, app),
			"policy": tftypes.NewValue(tftypes.String, policy),
		}),
	}
}

func newTestPlan(t *testing.T, app, policy string) tfsdk.Plan {
	t.Helper()
	s := testAppAccessSchema(t)
	return tfsdk.Plan{
		Schema: s.Schema,
		Raw: tftypes.NewValue(tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"app":    tftypes.String,
				"policy": tftypes.String,
			},
		}, map[string]tftypes.Value{
			"app":    tftypes.NewValue(tftypes.String, app),
			"policy": tftypes.NewValue(tftypes.String, policy),
		}),
	}
}

func testAppAccessSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()
	r := NewAppAccessResource()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics.Errors())
	}
	return *resp
}

func emptyState(t *testing.T) tfsdk.State {
	t.Helper()
	s := testAppAccessSchema(t)
	return tfsdk.State{Schema: s.Schema}
}

func TestAppAccessResource_Create_Success(t *testing.T) {
	var capturedReq *authorizer.AssignIdentityRequest
	mock := &mockAuthorizerClient{
		assignFn: func(ctx context.Context, req *authorizer.AssignIdentityRequest) (*authorizer.AssignIdentityResponse, error) {
			capturedReq = req
			return &authorizer.AssignIdentityResponse{}, nil
		},
	}

	r := &AppAccessResource{conn: mock, org: "test-org"}

	plan := newTestPlan(t, "my-app", "my-policy")
	schema := testAppAccessSchema(t)
	req := resource.CreateRequest{Plan: plan}
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}

	r.Create(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create() returned errors: %v", resp.Diagnostics.Errors())
	}

	if capturedReq == nil {
		t.Fatal("AssignIdentity was not called")
	}
	if capturedReq.Organization != "test-org" {
		t.Errorf("Expected org 'test-org', got '%s'", capturedReq.Organization)
	}
	appId := capturedReq.Identity.GetApplicationId()
	if appId == nil {
		t.Fatal("Expected ApplicationId identity, got nil")
	}
	if appId.Subject != "my-app" {
		t.Errorf("Expected app subject 'my-app', got '%s'", appId.Subject)
	}
	policyId := capturedReq.GetPolicyId()
	if policyId == nil {
		t.Fatal("Expected PolicyId assignment, got nil")
	}
	if policyId.Name != "my-policy" {
		t.Errorf("Expected policy 'my-policy', got '%s'", policyId.Name)
	}

	var data AppAccessResourceModel
	resp.State.Get(context.Background(), &data)
	if data.App.ValueString() != "my-app" {
		t.Errorf("Expected state app 'my-app', got '%s'", data.App.ValueString())
	}
}

func TestAppAccessResource_Create_Error(t *testing.T) {
	mock := &mockAuthorizerClient{
		assignFn: func(ctx context.Context, req *authorizer.AssignIdentityRequest) (*authorizer.AssignIdentityResponse, error) {
			return nil, status.Error(codes.Internal, "server error")
		},
	}

	r := &AppAccessResource{conn: mock, org: "test-org"}

	plan := newTestPlan(t, "my-app", "my-policy")
	schema := testAppAccessSchema(t)
	req := resource.CreateRequest{Plan: plan}
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}

	r.Create(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("Expected Create() to return an error")
	}
}

func TestAppAccessResource_Read_Success(t *testing.T) {
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			appId := req.Identity.GetApplicationId()
			if appId == nil || appId.Subject != "my-app" {
				t.Errorf("Expected app 'my-app' in read request")
			}
			return &authorizer.GetIdentityAssignmentResponse{
				IdentityAssignment: &authorizer.IdentityAssignment{
					Policies: []*common.Policy{
						{Id: &common.PolicyIdentifier{Name: "my-policy", Organization: "test-org"}},
					},
				},
			}, nil
		},
	}

	r := &AppAccessResource{conn: mock, org: "test-org"}

	state := newTestState(t, "my-app", "my-policy")
	req := resource.ReadRequest{State: state}
	resp := &resource.ReadResponse{State: state}

	r.Read(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read() returned errors: %v", resp.Diagnostics.Errors())
	}

	var data AppAccessResourceModel
	resp.State.Get(context.Background(), &data)
	if data.App.ValueString() != "my-app" {
		t.Errorf("Expected state preserved, got app '%s'", data.App.ValueString())
	}
}

func TestAppAccessResource_Read_PolicyNoLongerAssigned(t *testing.T) {
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			return &authorizer.GetIdentityAssignmentResponse{
				IdentityAssignment: &authorizer.IdentityAssignment{
					Policies: []*common.Policy{
						{Id: &common.PolicyIdentifier{Name: "other-policy", Organization: "test-org"}},
					},
				},
			}, nil
		},
	}

	r := &AppAccessResource{conn: mock, org: "test-org"}

	state := newTestState(t, "my-app", "my-policy")
	req := resource.ReadRequest{State: state}
	resp := &resource.ReadResponse{State: state}

	r.Read(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read() should not return errors, got: %v", resp.Diagnostics.Errors())
	}

	if !resp.State.Raw.IsNull() {
		t.Error("Expected state to be removed when policy is no longer assigned")
	}
}

func TestAppAccessResource_Read_NotFound(t *testing.T) {
	mock := &mockAuthorizerClient{
		getAssignFn: func(ctx context.Context, req *authorizer.GetIdentityAssignmentRequest) (*authorizer.GetIdentityAssignmentResponse, error) {
			return nil, status.Error(codes.NotFound, "not found")
		},
	}

	r := &AppAccessResource{conn: mock, org: "test-org"}

	state := newTestState(t, "my-app", "my-policy")
	req := resource.ReadRequest{State: state}
	resp := &resource.ReadResponse{State: state}

	r.Read(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read() should not return errors on NotFound, got: %v", resp.Diagnostics.Errors())
	}

	// State should be removed (resource no longer exists)
	if !resp.State.Raw.IsNull() {
		t.Error("Expected state to be removed on NotFound")
	}
}

func TestAppAccessResource_Delete_Success(t *testing.T) {
	var capturedReq *authorizer.UnassignIdentityRequest
	mock := &mockAuthorizerClient{
		unassignFn: func(ctx context.Context, req *authorizer.UnassignIdentityRequest) (*authorizer.UnassignIdentityResponse, error) {
			capturedReq = req
			return &authorizer.UnassignIdentityResponse{}, nil
		},
	}

	r := &AppAccessResource{conn: mock, org: "test-org"}

	state := newTestState(t, "my-app", "my-policy")
	req := resource.DeleteRequest{State: state}
	resp := &resource.DeleteResponse{}

	r.Delete(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete() returned errors: %v", resp.Diagnostics.Errors())
	}

	if capturedReq == nil {
		t.Fatal("UnassignIdentity was not called")
	}
	appId := capturedReq.Identity.GetApplicationId()
	if appId == nil || appId.Subject != "my-app" {
		t.Error("Expected ApplicationId with subject 'my-app'")
	}
	policyId := capturedReq.GetPolicyId()
	if policyId == nil || policyId.Name != "my-policy" {
		t.Error("Expected PolicyId with name 'my-policy'")
	}
}

func TestAppAccessResource_Delete_NotFound(t *testing.T) {
	mock := &mockAuthorizerClient{
		unassignFn: func(ctx context.Context, req *authorizer.UnassignIdentityRequest) (*authorizer.UnassignIdentityResponse, error) {
			return nil, status.Error(codes.NotFound, "not found")
		},
	}

	r := &AppAccessResource{conn: mock, org: "test-org"}

	state := newTestState(t, "my-app", "my-policy")
	req := resource.DeleteRequest{State: state}
	resp := &resource.DeleteResponse{State: newTestState(t, "my-app", "my-policy")}

	r.Delete(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatal("Delete() should not error on NotFound")
	}
}

func TestAppAccessResource_Delete_Error(t *testing.T) {
	mock := &mockAuthorizerClient{
		unassignFn: func(ctx context.Context, req *authorizer.UnassignIdentityRequest) (*authorizer.UnassignIdentityResponse, error) {
			return nil, status.Error(codes.Internal, "server error")
		},
	}

	r := &AppAccessResource{conn: mock, org: "test-org"}

	state := newTestState(t, "my-app", "my-policy")
	req := resource.DeleteRequest{State: state}
	resp := &resource.DeleteResponse{State: newTestState(t, "my-app", "my-policy")}

	r.Delete(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("Expected Delete() to return an error")
	}
}

func TestAppAccessResource_Configure_NilProviderData(t *testing.T) {
	r := &AppAccessResource{}
	req := resource.ConfigureRequest{ProviderData: nil}
	resp := &resource.ConfigureResponse{}

	r.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatal("Configure() with nil provider data should not error")
	}
}

func TestAppAccessResource_Configure_WrongType(t *testing.T) {
	r := &AppAccessResource{}
	req := resource.ConfigureRequest{ProviderData: "wrong-type"}
	resp := &resource.ConfigureResponse{}

	r.Configure(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("Configure() with wrong type should error")
	}
}

func TestAppAccessResource_Create_UsesApplicationIdentity(t *testing.T) {
	mock := &mockAuthorizerClient{
		assignFn: func(ctx context.Context, req *authorizer.AssignIdentityRequest) (*authorizer.AssignIdentityResponse, error) {
			// Verify it's using ApplicationId, not UserId
			if req.Identity.GetUserId() != nil {
				t.Error("Expected ApplicationId identity, got UserId")
			}
			if req.Identity.GetApplicationId() == nil {
				t.Error("Expected ApplicationId identity, got nil")
			}
			return &authorizer.AssignIdentityResponse{}, nil
		},
	}

	r := &AppAccessResource{conn: mock, org: "test-org"}

	plan := newTestPlan(t, "my-app", "my-policy")
	schema := testAppAccessSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}
	r.Create(context.Background(), resource.CreateRequest{Plan: plan}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

// Verify the Create sets organization on both identity and policy
func TestAppAccessResource_Create_SetsOrganization(t *testing.T) {
	mock := &mockAuthorizerClient{
		assignFn: func(ctx context.Context, req *authorizer.AssignIdentityRequest) (*authorizer.AssignIdentityResponse, error) {
			if req.Organization != "acme-corp" {
				t.Errorf("Expected request org 'acme-corp', got '%s'", req.Organization)
			}
			if req.GetPolicyId().Organization != "acme-corp" {
				t.Errorf("Expected policy org 'acme-corp', got '%s'", req.GetPolicyId().Organization)
			}
			return &authorizer.AssignIdentityResponse{}, nil
		},
	}

	r := &AppAccessResource{conn: mock, org: "acme-corp"}

	plan := newTestPlan(t, "app", "policy")
	schema := testAppAccessSchema(t)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}
	r.Create(context.Background(), resource.CreateRequest{Plan: plan}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

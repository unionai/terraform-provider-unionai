package provider

import (
	"context"
	"testing"

	"github.com/flyteorg/flyte/flyteidl/gen/pb-go/flyteidl/admin"
	"github.com/flyteorg/flyte/flyteidl/gen/pb-go/flyteidl/service"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockAdminClient implements the subset of service.AdminServiceClient used by
// ProjectDomainAttributesResource. The embedded interface satisfies the rest.
type mockAdminClient struct {
	service.AdminServiceClient
	updateFn func(ctx context.Context, req *admin.ProjectDomainAttributesUpdateRequest) (*admin.ProjectDomainAttributesUpdateResponse, error)
	getFn    func(ctx context.Context, req *admin.ProjectDomainAttributesGetRequest) (*admin.ProjectDomainAttributesGetResponse, error)
	deleteFn func(ctx context.Context, req *admin.ProjectDomainAttributesDeleteRequest) (*admin.ProjectDomainAttributesDeleteResponse, error)
}

func (m *mockAdminClient) UpdateProjectDomainAttributes(ctx context.Context, in *admin.ProjectDomainAttributesUpdateRequest, opts ...grpc.CallOption) (*admin.ProjectDomainAttributesUpdateResponse, error) {
	return m.updateFn(ctx, in)
}

func (m *mockAdminClient) GetProjectDomainAttributes(ctx context.Context, in *admin.ProjectDomainAttributesGetRequest, opts ...grpc.CallOption) (*admin.ProjectDomainAttributesGetResponse, error) {
	return m.getFn(ctx, in)
}

func (m *mockAdminClient) DeleteProjectDomainAttributes(ctx context.Context, in *admin.ProjectDomainAttributesDeleteRequest, opts ...grpc.CallOption) (*admin.ProjectDomainAttributesDeleteResponse, error) {
	return m.deleteFn(ctx, in)
}

func TestProjectDomainAttributesResource_Metadata(t *testing.T) {
	r := NewProjectDomainAttributesResource()
	req := resource.MetadataRequest{ProviderTypeName: "unionai"}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), req, resp)

	if resp.TypeName != "unionai_project_domain_attributes" {
		t.Errorf("Expected type name 'unionai_project_domain_attributes', got '%s'", resp.TypeName)
	}
}

func TestProjectDomainAttributesResource_Schema(t *testing.T) {
	r := NewProjectDomainAttributesResource()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics.Errors())
	}
	for _, attr := range []string{"id", "project", "domain", "attributes"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("Expected '%s' attribute in schema", attr)
		}
	}
}

func projectDomainAttributesSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()
	r := NewProjectDomainAttributesResource()
	resp := resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return resp
}

func objectType() tftypes.Object {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":         tftypes.String,
			"project":    tftypes.String,
			"domain":     tftypes.String,
			"attributes": tftypes.Map{ElementType: tftypes.String},
		},
	}
}

func newPDAPlan(t *testing.T, project, domain string, attrs map[string]string) tfsdk.Plan {
	t.Helper()
	s := projectDomainAttributesSchema(t)
	attrVals := make(map[string]tftypes.Value, len(attrs))
	for k, v := range attrs {
		attrVals[k] = tftypes.NewValue(tftypes.String, v)
	}
	return tfsdk.Plan{
		Schema: s.Schema,
		Raw: tftypes.NewValue(objectType(), map[string]tftypes.Value{
			"id":         tftypes.NewValue(tftypes.String, nil),
			"project":    tftypes.NewValue(tftypes.String, project),
			"domain":     tftypes.NewValue(tftypes.String, domain),
			"attributes": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, attrVals),
		}),
	}
}

func newPDAState(t *testing.T, project, domain string, attrs map[string]string) tfsdk.State {
	t.Helper()
	s := projectDomainAttributesSchema(t)
	attrVals := make(map[string]tftypes.Value, len(attrs))
	for k, v := range attrs {
		attrVals[k] = tftypes.NewValue(tftypes.String, v)
	}
	return tfsdk.State{
		Schema: s.Schema,
		Raw: tftypes.NewValue(objectType(), map[string]tftypes.Value{
			"id":         tftypes.NewValue(tftypes.String, project+"/"+domain),
			"project":    tftypes.NewValue(tftypes.String, project),
			"domain":     tftypes.NewValue(tftypes.String, domain),
			"attributes": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, attrVals),
		}),
	}
}

func TestProjectDomainAttributesResource_Create(t *testing.T) {
	var captured *admin.ProjectDomainAttributesUpdateRequest
	r := &ProjectDomainAttributesResource{
		conn: &mockAdminClient{
			updateFn: func(ctx context.Context, req *admin.ProjectDomainAttributesUpdateRequest) (*admin.ProjectDomainAttributesUpdateResponse, error) {
				captured = req
				return &admin.ProjectDomainAttributesUpdateResponse{}, nil
			},
		},
	}

	resp := &resource.CreateResponse{State: tfsdk.State{Schema: projectDomainAttributesSchema(t).Schema}}
	r.Create(context.Background(), resource.CreateRequest{
		Plan: newPDAPlan(t, "myproj", "development", map[string]string{"defaultUserRoleValue": "arn:aws:iam::123:role/x"}),
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create() errors: %v", resp.Diagnostics.Errors())
	}
	if captured == nil {
		t.Fatal("UpdateProjectDomainAttributes was not called")
	}
	pda := captured.GetAttributes()
	if pda.GetProject() != "myproj" || pda.GetDomain() != "development" {
		t.Errorf("unexpected request scope: %+v", pda)
	}
	cra := pda.GetMatchingAttributes().GetClusterResourceAttributes()
	if cra == nil || cra.GetAttributes()["defaultUserRoleValue"] != "arn:aws:iam::123:role/x" {
		t.Errorf("unexpected cluster resource attributes: %+v", cra)
	}
}

func TestProjectDomainAttributesResource_Read(t *testing.T) {
	r := &ProjectDomainAttributesResource{
		conn: &mockAdminClient{
			getFn: func(ctx context.Context, req *admin.ProjectDomainAttributesGetRequest) (*admin.ProjectDomainAttributesGetResponse, error) {
				if req.GetResourceType() != admin.MatchableResource_CLUSTER_RESOURCE {
					t.Errorf("expected CLUSTER_RESOURCE, got %v", req.GetResourceType())
				}
				return &admin.ProjectDomainAttributesGetResponse{
					Attributes: &admin.ProjectDomainAttributes{
						Project: "myproj",
						Domain:  "development",
						MatchingAttributes: &admin.MatchingAttributes{
							Target: &admin.MatchingAttributes_ClusterResourceAttributes{
								ClusterResourceAttributes: &admin.ClusterResourceAttributes{
									Attributes: map[string]string{"defaultUserRoleValue": "arn:aws:iam::123:role/x"},
								},
							},
						},
					},
				}, nil
			},
		},
	}

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: projectDomainAttributesSchema(t).Schema}}
	r.Read(context.Background(), resource.ReadRequest{
		State: newPDAState(t, "myproj", "development", map[string]string{"defaultUserRoleValue": "stale"}),
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read() errors: %v", resp.Diagnostics.Errors())
	}
	var out ProjectDomainAttributesResourceModel
	resp.State.Get(context.Background(), &out)
	attrs := out.Attributes.Elements()
	if len(attrs) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(attrs))
	}
}

func TestProjectDomainAttributesResource_ReadNotFound(t *testing.T) {
	r := &ProjectDomainAttributesResource{
		conn: &mockAdminClient{
			getFn: func(ctx context.Context, req *admin.ProjectDomainAttributesGetRequest) (*admin.ProjectDomainAttributesGetResponse, error) {
				return nil, status.Error(codes.NotFound, "not found")
			},
		},
	}

	resp := &resource.ReadResponse{State: tfsdk.State{Schema: projectDomainAttributesSchema(t).Schema}}
	resp.State.Raw = newPDAState(t, "myproj", "development", map[string]string{"k": "v"}).Raw
	r.Read(context.Background(), resource.ReadRequest{
		State: newPDAState(t, "myproj", "development", map[string]string{"k": "v"}),
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read() errors: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed on NotFound")
	}
}

func TestProjectDomainAttributesResource_Delete(t *testing.T) {
	called := false
	r := &ProjectDomainAttributesResource{
		conn: &mockAdminClient{
			deleteFn: func(ctx context.Context, req *admin.ProjectDomainAttributesDeleteRequest) (*admin.ProjectDomainAttributesDeleteResponse, error) {
				called = true
				if req.GetResourceType() != admin.MatchableResource_CLUSTER_RESOURCE {
					t.Errorf("expected CLUSTER_RESOURCE, got %v", req.GetResourceType())
				}
				return &admin.ProjectDomainAttributesDeleteResponse{}, nil
			},
		},
	}

	resp := &resource.DeleteResponse{State: tfsdk.State{Schema: projectDomainAttributesSchema(t).Schema}}
	r.Delete(context.Background(), resource.DeleteRequest{
		State: newPDAState(t, "myproj", "development", map[string]string{"k": "v"}),
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete() errors: %v", resp.Diagnostics.Errors())
	}
	if !called {
		t.Error("DeleteProjectDomainAttributes was not called")
	}
}

func TestProjectDomainAttributesResource_ImportState(t *testing.T) {
	r := &ProjectDomainAttributesResource{}
	cases := []struct {
		id      string
		wantErr bool
	}{
		{"myproj/development", false},
		{"myproj", true},
		{"/development", true},
		{"myproj/", true},
		{"", true},
	}
	for _, tc := range cases {
		resp := &resource.ImportStateResponse{State: tfsdk.State{
			Schema: projectDomainAttributesSchema(t).Schema,
			Raw:    tftypes.NewValue(objectType(), nil),
		}}
		r.ImportState(context.Background(), resource.ImportStateRequest{ID: tc.id}, resp)
		if got := resp.Diagnostics.HasError(); got != tc.wantErr {
			t.Errorf("ImportState(%q): wantErr=%v gotErr=%v (%v)", tc.id, tc.wantErr, got, resp.Diagnostics.Errors())
		}
	}
}

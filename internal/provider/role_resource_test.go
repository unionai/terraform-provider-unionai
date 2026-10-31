package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/unionai/cloud/gen/pb-go/authorizer"
	"github.com/unionai/cloud/gen/pb-go/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockRoleClient implements the subset of authorizer.AuthorizerServiceClient used by RoleResource.
type mockRoleClient struct {
	authorizer.AuthorizerServiceClient
	calls []string
	roles []*common.Role
}

func (m *mockRoleClient) GetRole(ctx context.Context, in *authorizer.GetRoleRequest, opts ...grpc.CallOption) (*authorizer.GetRoleResponse, error) {
	return nil, status.Error(codes.NotFound, "not found")
}

func (m *mockRoleClient) CreateRole(ctx context.Context, in *authorizer.CreateRoleRequest, opts ...grpc.CallOption) (*authorizer.CreateRoleResponse, error) {
	m.roles = append(m.roles, in.Role)
	return &authorizer.CreateRoleResponse{}, nil
}

func (m *mockRoleClient) CreateRoleAction(ctx context.Context, in *authorizer.CreateRoleActionRequest, opts ...grpc.CallOption) (*authorizer.CreateRoleActionResponse, error) {
	m.calls = append(m.calls, "add "+in.Id.Name+" "+in.Action.String())
	return &authorizer.CreateRoleActionResponse{}, nil
}

func (m *mockRoleClient) DeleteRoleAction(ctx context.Context, in *authorizer.DeleteRoleActionRequest, opts ...grpc.CallOption) (*authorizer.DeleteRoleActionResponse, error) {
	m.calls = append(m.calls, "remove "+in.Id.Name+" "+in.Action.String())
	return &authorizer.DeleteRoleActionResponse{}, nil
}

func testRoleSchema(t *testing.T) schema.Schema {
	t.Helper()
	resp := &resource.SchemaResponse{}
	NewRoleResource().Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned errors: %v", resp.Diagnostics.Errors())
	}
	return resp.Schema
}

func testRoleModel(actions ...string) RoleResourceModel {
	return RoleResourceModel{
		Id:          types.StringValue("event-driven-executor"),
		Name:        types.StringValue("event-driven-executor"),
		Description: types.StringNull(),
		Actions:     convertStringsToSet(actions),
	}
}

func newRolePlan(t *testing.T, m RoleResourceModel) tfsdk.Plan {
	t.Helper()
	plan := tfsdk.Plan{Schema: testRoleSchema(t)}
	if diags := plan.Set(context.Background(), &m); diags.HasError() {
		t.Fatalf("plan.Set() returned errors: %v", diags.Errors())
	}
	return plan
}

func newRoleState(t *testing.T, m RoleResourceModel) tfsdk.State {
	t.Helper()
	state := tfsdk.State{Schema: testRoleSchema(t)}
	if diags := state.Set(context.Background(), &m); diags.HasError() {
		t.Fatalf("state.Set() returned errors: %v", diags.Errors())
	}
	return state
}

func TestRoleResource_Schema_ActionsUpdateInPlace(t *testing.T) {
	actions := testRoleSchema(t).Attributes["actions"].(schema.SetAttribute)
	if len(actions.PlanModifiers) != 0 {
		t.Errorf("Expected no plan modifiers on actions, got %d", len(actions.PlanModifiers))
	}
}

func TestRoleResource_Create_NewActions(t *testing.T) {
	mock := &mockRoleClient{}
	r := &RoleResource{conn: mock, org: "test-org"}
	m := testRoleModel("view_apps", "manage_secrets", "view_permissions")
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: testRoleSchema(t)}}

	r.Create(context.Background(), resource.CreateRequest{Plan: newRolePlan(t, m)}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create() returned errors: %v", resp.Diagnostics.Errors())
	}
	want := map[common.Action]bool{
		common.Action_ACTION_VIEW_APPS:        true,
		common.Action_ACTION_MANAGE_SECRETS:   true,
		common.Action_ACTION_VIEW_PERMISSIONS: true,
	}
	got := mock.roles[0].Actions
	if len(got) != len(want) {
		t.Fatalf("Expected %d actions, got %v", len(want), got)
	}
	for _, a := range got {
		if !want[a] {
			t.Errorf("Unexpected action %s", a)
		}
	}
}

func TestRoleResource_Update_AddsBeforeRemoving(t *testing.T) {
	mock := &mockRoleClient{}
	r := &RoleResource{conn: mock, org: "test-org"}
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: testRoleSchema(t)}}

	r.Update(context.Background(), resource.UpdateRequest{
		Plan:  newRolePlan(t, testRoleModel("view_flyte_executions", "view_apps")),
		State: newRoleState(t, testRoleModel("view_flyte_executions", "administer_project")),
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update() returned errors: %v", resp.Diagnostics.Errors())
	}
	want := []string{
		"add event-driven-executor ACTION_VIEW_APPS",
		"remove event-driven-executor ACTION_ADMINISTER_PROJECT",
	}
	if len(mock.calls) != len(want) || mock.calls[0] != want[0] || mock.calls[1] != want[1] {
		t.Errorf("Expected calls %v, got %v", want, mock.calls)
	}
	var data RoleResourceModel
	resp.State.Get(context.Background(), &data)
	if !data.Actions.Equal(convertStringsToSet([]string{"view_flyte_executions", "view_apps"})) {
		t.Errorf("Expected planned actions in state, got %v", data.Actions)
	}
}

func TestRoleResource_Update_UnknownActionMakesNoCalls(t *testing.T) {
	mock := &mockRoleClient{}
	r := &RoleResource{conn: mock, org: "test-org"}
	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: testRoleSchema(t)}}

	r.Update(context.Background(), resource.UpdateRequest{
		Plan:  newRolePlan(t, testRoleModel("view_flyte_executions", "view_everything")),
		State: newRoleState(t, testRoleModel("view_flyte_executions")),
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("Expected an error for an unknown action")
	}
	if len(mock.calls) != 0 {
		t.Errorf("Expected no API calls, got %v", mock.calls)
	}
}

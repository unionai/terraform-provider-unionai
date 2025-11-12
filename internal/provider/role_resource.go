package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/unionai/cloud/gen/pb-go/authorizer"
	"github.com/unionai/cloud/gen/pb-go/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

// RoleResource defines the resource implementation.
type RoleResource struct {
	conn authorizer.AuthorizerServiceClient
	org  string
}

// RoleResourceModel describes the resource data model.
type RoleResourceModel struct {
	Id          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Actions     types.Set    `tfsdk:"actions"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Role resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Role name",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Role description",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"actions": schema.SetAttribute{
				MarkdownDescription: "Policy actions",
				Required:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *RoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*providerContext)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *providerContext, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.conn = authorizer.NewAuthorizerServiceClient(client.conn)
	if r.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *authorizer.AuthorizerServiceClient, got: %T. Please report this issue to the provider developers.", r.conn),
		)
		return
	}
	r.org = client.org
}

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = data.Name

	// Prevent role from being overridden if it already exists
	if _, err := r.conn.GetRole(ctx, &authorizer.GetRoleRequest{
		Id: &common.RoleIdentifier{
			Name:         data.Id.ValueString(),
			Organization: r.org,
		},
	}); err == nil {
		resp.Diagnostics.AddError("Role already exists", fmt.Sprintf("Role %s already exists", data.Id.ValueString()))
		return
	}

	actionErrors := false
	createRequest := &authorizer.CreateRoleRequest{
		Role: &common.Role{
			Id: &common.RoleIdentifier{
				Name:         data.Id.ValueString(),
				Organization: r.org,
			},
			RoleSpec: &common.RoleSpec{
				Description: data.Description.ValueString(),
			},
			RoleType: common.RoleType_ROLE_TYPE_CUSTOM,
			Actions: func() []common.Action {
				var actions []string
				if diag := data.Actions.ElementsAs(ctx, &actions, false); diag.HasError() {
					return nil
				}
				out := make([]common.Action, len(actions))
				for i, a := range actions {
					action := common.Action_value[strings.ToUpper(fmt.Sprintf("action_%s", a))]
					if action == int32(common.Action_ACTION_NONE) {
						resp.Diagnostics.AddError("Action does not exist", fmt.Sprintf("Cannot find action: %s", a))
						actionErrors = true
					}
					out[i] = common.Action(action)
				}
				return out
			}(),
		},
	}

	if actionErrors {
		return
	}

	tflog.Debug(ctx, "CreateRole request", map[string]interface{}{
		"role(create)": createRequest.Role,
	})
	_, err := r.conn.CreateRole(ctx, createRequest)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create role, got error: %s", err))
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.conn.GetRole(ctx, &authorizer.GetRoleRequest{
		Id: &common.RoleIdentifier{
			Name:         data.Id.ValueString(),
			Organization: r.org,
		},
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read role, got error: %s", err))
		return
	}

	data.Id = types.StringValue(role.Role.Id.Name)
	data.Name = types.StringValue(role.Role.Id.Name)
	if role.Role.RoleSpec != nil {
		data.Description = types.StringValue(role.Role.RoleSpec.Description)
	}
	actions := make([]attr.Value, len(role.Role.Actions))
	for i, a := range role.Role.Actions {
		actions[i] = types.StringValue(strings.ToLower(strings.ReplaceAll(common.Action_name[int32(a)], "ACTION_", "")))
	}
	data.Actions = types.SetValueMust(types.StringType, actions)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.conn.DeleteRole(ctx, &authorizer.DeleteRoleRequest{
		Id: &common.RoleIdentifier{
			Name:         data.Id.ValueString(),
			Organization: r.org,
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete role, got error: %s", err))
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/unionai/cloud/gen/pb-go/authorizer"
	"github.com/unionai/cloud/gen/pb-go/common"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &RoleDataSource{}

func NewRoleDataSource() datasource.DataSource {
	return &RoleDataSource{}
}

// RoleDataSource defines the data source implementation.
type RoleDataSource struct {
	conn authorizer.AuthorizerServiceClient
	org  string
}

type ActionName string

const (
	ACTION_VIEW_FLYTE_INVENTORY              ActionName = "view_flyte_inventory"
	ACTION_VIEW_FLYTE_EXECUTIONS             ActionName = "view_flyte_executions"
	ACTION_REGISTER_FLYTE_INVENTORY          ActionName = "register_flyte_inventory"
	ACTION_CREATE_FLYTE_EXECUTIONS           ActionName = "create_flyte_executions"
	ACTION_ADMINISTER_PROJECT                ActionName = "administer_project"
	ACTION_MANAGE_PERMISSIONS                ActionName = "manage_permissions"
	ACTION_ADMINISTER_ACCOUNT                ActionName = "administer_account"
	ACTION_MANAGE_CLUSTER                    ActionName = "manage_cluster"
	ACTION_EDIT_EXECUTION_RELATED_ATTRIBUTES ActionName = "edit_execution_related_attributes"
	ACTION_EDIT_CLUSTER_RELATED_ATTRIBUTES   ActionName = "edit_cluster_related_attributes"
	ACTION_EDIT_UNUSED_ATTRIBUTES            ActionName = "edit_unused_attributes"
	ACTION_SUPPORT_SYSTEM_LOGS               ActionName = "support_system_logs"
	ACTION_UNKNOWN                           ActionName = "unknown"
)

// RoleDataSourceModel describes the data source data model.
type RoleDataSourceModel struct {
	Id      types.String `tfsdk:"id"`
	Actions types.List   `tfsdk:"actions"`
}

func (d *RoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *RoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Role data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Role identifier",
				Required:            true,
			},
			"actions": schema.ListAttribute{
				MarkdownDescription: "List of actions associated with the role",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

func (d *RoleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.conn = authorizer.NewAuthorizerServiceClient(client.conn)
	if d.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *authorizer.AuthorizerServiceClient, got: %T. Please report this issue to the provider developers.", d.conn),
		)
		return
	}
	d.org = client.org
}

func (d *RoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RoleDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	role, err := d.conn.GetRole(context.Background(), &authorizer.GetRoleRequest{Id: &common.RoleIdentifier{Name: data.Id.ValueString(), Organization: d.org}})
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch role", err.Error())
		return
	}
	tflog.Trace(ctx, "GetRole response", map[string]interface{}{"role": role})

	actions := make([]ActionName, 0, len(role.Role.Actions))
	for _, action := range role.Role.Actions {
		var _action ActionName
		switch action {
		case common.Action_ACTION_VIEW_FLYTE_INVENTORY:
			_action = ACTION_VIEW_FLYTE_INVENTORY
		case common.Action_ACTION_VIEW_FLYTE_EXECUTIONS:
			_action = ACTION_VIEW_FLYTE_EXECUTIONS
		case common.Action_ACTION_REGISTER_FLYTE_INVENTORY:
			_action = ACTION_REGISTER_FLYTE_INVENTORY
		case common.Action_ACTION_CREATE_FLYTE_EXECUTIONS:
			_action = ACTION_CREATE_FLYTE_EXECUTIONS
		case common.Action_ACTION_ADMINISTER_PROJECT:
			_action = ACTION_ADMINISTER_PROJECT
		case common.Action_ACTION_MANAGE_PERMISSIONS:
			_action = ACTION_MANAGE_PERMISSIONS
		case common.Action_ACTION_ADMINISTER_ACCOUNT:
			_action = ACTION_ADMINISTER_ACCOUNT
		case common.Action_ACTION_MANAGE_CLUSTER:
			_action = ACTION_MANAGE_CLUSTER
		case common.Action_ACTION_EDIT_CLUSTER_RELATED_ATTRIBUTES:
			_action = ACTION_EDIT_CLUSTER_RELATED_ATTRIBUTES
		case common.Action_ACTION_EDIT_EXECUTION_RELATED_ATTRIBUTES:
			_action = ACTION_EDIT_EXECUTION_RELATED_ATTRIBUTES
		case common.Action_ACTION_EDIT_UNUSED_ATTRIBUTES:
			_action = ACTION_EDIT_UNUSED_ATTRIBUTES
		case common.Action_ACTION_SUPPORT_SYSTEM_LOGS:
			_action = ACTION_SUPPORT_SYSTEM_LOGS
		default:
			_action = ACTION_UNKNOWN
		}

		actions = append(actions, _action)
	}

	// Convert []string → types.List
	listValue, diag := types.ListValueFrom(ctx, types.StringType, actions)
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Actions = listValue
	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "read a data source")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

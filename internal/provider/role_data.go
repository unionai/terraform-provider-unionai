package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/unionai/terraform-provider-unionai/proto/authorizer"
	"github.com/unionai/terraform-provider-unionai/proto/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// RoleDataSourceModel describes the data source data model.
type RoleDataSourceModel struct {
	Id      types.String `tfsdk:"id"`
	Actions types.Set    `tfsdk:"actions"`
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
			"actions": schema.SetAttribute{
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
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddError("Role not found", fmt.Sprintf("Role with ID %s not found", data.Id.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to fetch role", err.Error())
		return
	}
	tflog.Trace(ctx, "GetRole response", map[string]interface{}{"role": role})

	data.Actions = convertArrayToSetGetter(role.Role.Actions, func(a common.Action) string {
		return strings.ToLower(strings.ReplaceAll(common.Action_name[int32(a)], "ACTION_", ""))
	})

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

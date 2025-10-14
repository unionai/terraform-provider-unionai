package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/unionai/cloud/gen/pb-go/common"
	"github.com/unionai/cloud/gen/pb-go/identity"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &UserAccessDataSource{}

func NewUserAccessDataSource() datasource.DataSource {
	return &UserAccessDataSource{}
}

// UserAccessDataSource defines the data source implementation.
type UserAccessDataSource struct {
	conn identity.UserServiceClient
	org  string
}

// UserAccessDataSourceModel describes the data source data model.
type UserAccessDataSourceModel struct {
	UserId   types.String `tfsdk:"user_id"`
	PolicyId types.String `tfsdk:"policy_id"`
}

func (d *UserAccessDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_access"
}

func (d *UserAccessDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Policy Binding data source",

		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				MarkdownDescription: "User identifier",
				Required:            true,
			},
			"policy_id": schema.StringAttribute{
				MarkdownDescription: "Policy identifier",
				Required:            true,
			},
		},
	}
}

func (d *UserAccessDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.conn = identity.NewUserServiceClient(client.conn)
	if d.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *identity.UserServiceClient, got: %T. Please report this issue to the provider developers.", d.conn),
		)
		return
	}
	d.org = client.org
}

func (d *UserAccessDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserAccessDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	userResp, err := d.conn.GetUser(ctx, &identity.GetUserRequest{
		Id: &common.UserIdentifier{
			Subject: data.UserId.ValueString(),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch user", err.Error())
		return
	}
	user := userResp.User
	tflog.Trace(ctx, "Fetched user", map[string]interface{}{
		"user": user,
	})

	tflog.Trace(ctx, "Fetched user indentity policies", map[string]interface{}{
		"user_email": data.UserId.ValueString(),
		"policies":   user.Policies,
	})

	var assigned bool
	for _, p := range user.Policies {
		if p.Id.Name == data.PolicyId.ValueString() && p.Id.Organization == d.org {
			assigned = true
			break
		}
	}

	// Check if the user is assigned to the policy
	if !assigned {
		resp.Diagnostics.AddError("User not assigned to policy",
			fmt.Sprintf("User %s is not assigned to policy %s", data.UserId.ValueString(), data.PolicyId.ValueString()))
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

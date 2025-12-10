package provider

import (
	"context"
	"fmt"

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
var _ datasource.DataSource = &AppAccessDataSource{}

func NewAppAccessDataSource() datasource.DataSource {
	return &AppAccessDataSource{}
}

// AppAccessDataSource defines the data source implementation.
type AppAccessDataSource struct {
	conn authorizer.AuthorizerServiceClient
	org  string
}

// AppAccessDataSourceModel describes the data source data model.
type AppAccessDataSourceModel struct {
	AppId    types.String `tfsdk:"app_id"`
	PolicyId types.String `tfsdk:"policy_id"`
}

func (d *AppAccessDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_access"
}

func (d *AppAccessDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Policy Binding data source",

		Attributes: map[string]schema.Attribute{
			"app_id": schema.StringAttribute{
				MarkdownDescription: "Application identifier",
				Required:            true,
			},
			"policy_id": schema.StringAttribute{
				MarkdownDescription: "Policy identifier",
				Required:            true,
			},
		},
	}
}

func (d *AppAccessDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AppAccessDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AppAccessDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	app, err := d.conn.GetIdentityAssignments(ctx, &authorizer.GetIdentityAssignmentRequest{
		Organization: d.org,
		Identity: &common.Identity{
			Principal: &common.Identity_ApplicationId{
				ApplicationId: &common.ApplicationIdentifier{
					Subject: data.AppId.ValueString(),
				},
			},
		},
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddError("Application not found", fmt.Sprintf("Application with ID %s not found", data.AppId.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to fetch application", err.Error())
		return
	}
	tflog.Trace(ctx, "Fetched application indentity policies", map[string]interface{}{
		"app_id":      data.AppId.ValueString(),
		"assignments": app,
	})

	var assigned bool
	for _, p := range app.IdentityAssignment.Policies {
		if p.Id.Name == data.PolicyId.ValueString() && p.Id.Organization == d.org {
			assigned = true
			break
		}
	}

	// Check if the user is assigned to the policy
	if !assigned {
		resp.Diagnostics.AddError("Application not assigned to policy",
			fmt.Sprintf("Application %s is not assigned to policy %s", data.AppId.ValueString(), data.PolicyId.ValueString()))
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

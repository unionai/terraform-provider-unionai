package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/unionai/cloud/gen/pb-go/identity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ApiKeyDataSource{}

func NewApiKeyDataSource() datasource.DataSource {
	return &ApiKeyDataSource{}
}

// ApiKeyDataSource defines the data source implementation.
type ApiKeyDataSource struct {
	conn identity.AppsServiceClient
	org  string
}

// ApiKeyDataSourceModel describes the data source data model.
type ApiKeyDataSourceModel struct {
	Id types.String `tfsdk:"id"`
}

func (d *ApiKeyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (d *ApiKeyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "API key data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "API key identifier",
				Required:            true,
			},
		},
	}
}

func (d *ApiKeyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.conn = identity.NewAppsServiceClient(client.conn)
	if d.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *identity.AppsServiceClient, got: %T. Please report this issue to the provider developers.", d.conn),
		)
		return
	}
	d.org = client.org
}

func (d *ApiKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ApiKeyDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := d.conn.Get(ctx, &identity.GetAppRequest{
		Organization: d.org,
		ClientId:     data.Id.ValueString(),
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddError("API key not found", fmt.Sprintf("API key with ID %s not found", data.Id.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to fetch API key", err.Error())
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

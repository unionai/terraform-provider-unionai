package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ControlplaneDataSource{}

func NewControlplaneDataSource() datasource.DataSource {
	return &ControlplaneDataSource{}
}

// ControlplaneDataSource defines the data source implementation.
type ControlplaneDataSource struct {
	org  string
	host string
}

// ControlplaneDataSourceModel describes the data source data model.
type ControlplaneDataSourceModel struct {
	Endpoint     types.String `tfsdk:"endpoint"`
	Host         types.String `tfsdk:"host"`
	Organization types.String `tfsdk:"organization"`
}

func (d *ControlplaneDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_controlplane"
}

func (d *ControlplaneDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Cluster data source",

		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Controlplane endpoint",
				Computed:            true,
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "Controlplane host",
				Computed:            true,
			},
			"organization": schema.StringAttribute{
				MarkdownDescription: "Controlplane organization",
				Computed:            true,
			},
		},
	}
}

func (d *ControlplaneDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.org = client.org
	d.host = client.host
}

func (d *ControlplaneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ControlplaneDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Endpoint = types.StringValue(fmt.Sprintf("https://%s", d.host))
	data.Host = types.StringValue(d.host)
	data.Organization = types.StringValue(d.org)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
